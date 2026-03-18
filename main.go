package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	deserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
)

func init() {
	// Add scheme
	corev1.AddToScheme(runtime.NewScheme())
	admissionv1.AddToScheme(runtime.NewScheme())
}

// 手动创建 sidecar 容器
func createSidecarContainer() corev1.Container {
	cpuLimit, _ := resource.ParseQuantity("100m")
	memLimit, _ := resource.ParseQuantity("128Mi")
	cpuReq, _ := resource.ParseQuantity("50m")
	memReq, _ := resource.ParseQuantity("64Mi")

	return corev1.Container{
		Name:  "sidecar-injected",
		Image: "busybox:latest",
		Args:  []string{"sh", "-c", "while true; do echo 'Sidecar running at '$(date); sleep 10; done"},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    cpuLimit,
				corev1.ResourceMemory: memLimit,
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    cpuReq,
				corev1.ResourceMemory: memReq,
			},
		},
	}
}

// handleAdmission 处理 admission webhook 请求
func handleAdmission(w http.ResponseWriter, r *http.Request) {
	log.Println("=== 收到 Webhook 请求 ===")

	// 读取请求体
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("读取请求体失败: %v\n", err)
		http.Error(w, fmt.Sprintf("读取请求体失败: %v", err), http.StatusBadRequest)
		return
	}

	// 打印原始请求
	log.Printf("原始请求体:\n%s\n", string(body))

	// 解析 AdmissionReview
	admissionReview := admissionv1.AdmissionReview{}
	_, _, err = deserializer.Decode(body, nil, &admissionReview)
	if err != nil {
		log.Printf("解析请求失败: %v\n", err)
		http.Error(w, fmt.Sprintf("解析请求失败: %v", err), http.StatusBadRequest)
		return
	}

	// 打印解析后的信息
	log.Printf("请求 UID: %s\n", admissionReview.Request.UID)
	log.Printf("请求操作: %s\n", admissionReview.Request.Operation)
	log.Printf("请求资源: %s/%s\n", admissionReview.Request.Resource.Group, admissionReview.Request.Resource.Resource)
	log.Printf("Pod 命名空间: %s\n", admissionReview.Request.Namespace)

	// 获取 Pod 信息
	pod := corev1.Pod{}
	err = json.Unmarshal(admissionReview.Request.Object.Raw, &pod)
	if err != nil {
		log.Printf("解析 Pod 失败: %v\n", err)
		http.Error(w, fmt.Sprintf("解析 Pod 失败: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("Pod 名称: %s\n", pod.Name)
	log.Printf("Pod 初始容器数: %d\n", len(pod.Spec.Containers))

	// 打印现有容器信息
	for i, container := range pod.Spec.Containers {
		log.Printf("  容器 %d: %s (镜像: %s)\n", i, container.Name, container.Image)
	}

	// 创建 patch 以注入 sidecar
	sidecar := createSidecarContainer()
	log.Printf("准备注入 sidecar: %s\n", sidecar.Name)

	// 构建 JSON patch
	patch := []map[string]interface{}{
		{
			"op":    "add",
			"path":  "/spec/containers/-",
			"value": sidecar,
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		log.Printf("创建 patch 失败: %v\n", err)
		http.Error(w, fmt.Sprintf("创建 patch 失败: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("生成的 patch:\n%s\n", string(patchBytes))

	// 创建 admission 响应
	admissionResponse := &admissionv1.AdmissionResponse{
		UID:     admissionReview.Request.UID,
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}

	// 创建响应 AdmissionReview
	responseReview := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: admissionResponse,
	}

	// 返回响应
	respBytes, err := json.Marshal(responseReview)
	if err != nil {
		log.Printf("编码响应失败: %v\n", err)
		http.Error(w, fmt.Sprintf("编码响应失败: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("返回响应:\n%s\n", string(respBytes))
	log.Println("=== Webhook 请求处理完成 ===")
	log.Println("")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBytes)
}

// healthz 健康检查
func healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	// 注册 HTTP 处理器
	http.HandleFunc("/mutate", handleAdmission)
	http.HandleFunc("/healthz", healthz)

	// 启动 HTTPS 服务器
	log.Println("启动 Webhook 服务器在 0.0.0.0:8443")
	log.Println("使用 TLS 证书: /etc/webhook/certs/tls.crt")
	log.Println("使用 TLS 密钥: /etc/webhook/certs/tls.key")

	err := http.ListenAndServeTLS(
		":8443",
		"/etc/webhook/certs/tls.crt",
		"/etc/webhook/certs/tls.key",
		nil,
	)
	if err != nil {
		log.Fatalf("启动服务器失败: %v\n", err)
	}
}

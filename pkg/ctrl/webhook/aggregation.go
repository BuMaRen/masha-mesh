package webhook

import (
	"encoding/json"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

func containerPatch(containerName, imageTag string, commands []string) ([]byte, error) {
	// sidecar 默认资源配额：限制注入容器占用，避免影响业务主容器。
	memLimit, _ := resource.ParseQuantity("128Mi")
	cpuLimit, _ := resource.ParseQuantity("100m")
	memReq, _ := resource.ParseQuantity("64Mi")
	cpuReq, _ := resource.ParseQuantity("50m")

	// 构造要注入到 Pod.spec.containers 的 sidecar 容器定义。
	ctn := corev1.Container{
		Name:    containerName,
		Image:   imageTag,
		Command: commands,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{"cpu": cpuReq, "memory": memReq},
			Limits:   corev1.ResourceList{"cpu": cpuLimit, "memory": memLimit},
		},
	}

	// 生成 JSON Patch：
	// op=add, path=/spec/containers/- 表示在 containers 数组末尾追加一个容器。
	return json.Marshal([]map[string]any{
		{
			"op":    "add",
			"path":  "/spec/containers/-",
			"value": ctn,
		},
	})
}

// TODO: 参数需要改造，imageTag 和 commands 从 crd 中获取
func (s *WebhookServer) Aggregation(engine *gin.Engine, imageTag string, commands []string) {
	// /mutate: 处理 Kubernetes AdmissionReview 请求并返回 sidecar 注入补丁。
	engine.POST("/mutate", func(c *gin.Context) {
		admissionReview := admissionv1.AdmissionReview{}
		if err := c.ShouldBindJSON(&admissionReview); err != nil || admissionReview.Request == nil {
			c.JSON(http.StatusBadRequest, &admissionv1.AdmissionResponse{
				Allowed: false,
				Result:  &metav1.Status{Message: "invalid request"},
			})
			return
		}

		if admissionReview.APIVersion == "" {
			// 兼容未显式传入 apiVersion 的场景。
			admissionReview.APIVersion = "admission.k8s.io/v1"
		}

		response := &admissionv1.AdmissionResponse{
			UID:     admissionReview.Request.UID,
			Allowed: true, // 默认允许，除非发生错误。
		}

		if admissionReview.Request.Resource.Resource != string(corev1.ResourcePods) {
			c.JSON(http.StatusInternalServerError, &admissionv1.AdmissionResponse{
				Allowed: false,
				Result:  &metav1.Status{Message: "unsupported resource type"},
			})
			return
		}

		pod := &corev1.Pod{}
		if err := json.Unmarshal(admissionReview.Request.Object.Raw, pod); err != nil {
			c.JSON(http.StatusBadRequest, &admissionv1.AdmissionResponse{
				Allowed: false,
				Result:  &metav1.Status{Message: "invalid request object"},
			})
			return
		}

		// 检查 pod 是否有 masha.io/injection 标签
		injectionContainer, hasInjectionLabel := pod.Labels[s.injectionLabel]
		if hasInjectionLabel {
			if container := s.getContainerCache(injectionContainer); container != nil {
				klog.Infof("Pod %s/%s has %s=%s", pod.Namespace, pod.Name, s.injectionLabel, injectionContainer)
				pt := admissionv1.PatchTypeJSONPatch
				patch, err := containerPatch(container.Spec.Name, container.Spec.Image, container.Spec.Command)
				if err == nil {
					response.PatchType = &pt
					response.Patch = patch
				}
			}
		}
		admissionReview.Response = response
		c.JSON(http.StatusOK, admissionReview)
		klog.Infof("Processed AdmissionReview for Pod %s/%s, injection=%v", pod.Namespace, pod.Name, hasInjectionLabel)
	})
}

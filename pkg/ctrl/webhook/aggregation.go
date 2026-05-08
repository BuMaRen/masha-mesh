package webhook

import (
	"encoding/json"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"

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
		if err := c.ShouldBindJSON(&admissionReview); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if admissionReview.Request == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing admission review request"})
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

		// if imcomingObj isn't pod, we just allow it without patch, otherwise we will patch it with sidecar container.
		if admissionReview.Request.Resource.Resource == string(corev1.ResourcePods) {
			// 查看 pod 的 labels，看一下需要注入什么容器
			pod := &corev1.Pod{}
			if err := json.Unmarshal(admissionReview.Request.Object.Raw, pod); err != nil {
				klog.Errorf("Failed to unmarshal pod: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse pod"})
				return
			}

			// 检查 pod 是否有 masha.io/injection 标签
			injectionContainer, hasInjectionLabel := pod.Labels["masha.io/injection"]
			if hasInjectionLabel {
				container := s.getContainerCache(injectionContainer)
				if container == nil {
					klog.Errorf("Container %s not found in cache", injectionContainer)
					c.JSON(http.StatusBadRequest, gin.H{"error": "container not found"})
					return
				}
				klog.Infof("Pod %s/%s has masha.io/injection=%s", pod.Namespace, pod.Name, injectionContainer)
				pt := admissionv1.PatchTypeJSONPatch
				patch, err := containerPatch(container.Spec.Name, container.Spec.Image, container.Spec.Command)
				if err == nil {
					response.PatchType = &pt
					response.Patch = patch
				}
			} else {
				klog.Infof("Pod %s/%s doesn't have masha.io/injection label, skipping sidecar injection", pod.Namespace, pod.Name)
			}
		}
		admissionReview.Response = response

		c.JSON(http.StatusOK, admissionReview)
		klog.Infof("Handled admission review for %s/%s, operation: %s, user: %s", admissionReview.Request.Namespace, admissionReview.Request.Name, admissionReview.Request.Operation, admissionReview.Request.UserInfo.Username)
	})
}

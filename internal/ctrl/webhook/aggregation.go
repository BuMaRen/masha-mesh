package webhook

import (
	"encoding/json"
	"net/http"

	"github.com/BuMaRen/mesh/pkg/cache"
	"github.com/gin-gonic/gin"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// getPodFromAdmissionReview 的入参 admissionReview 提前检查有效性
func getPodFromAdmissionReview(admissionReview *admissionv1.AdmissionReview) (*corev1.Pod, bool) {
	if admissionReview == nil || admissionReview.Request == nil {
		klog.Warningf("invalid admissionReview: %v", admissionReview)
		return nil, false
	}
	admissionRequest := admissionReview.Request
	pod := corev1.Pod{}
	if err := json.Unmarshal(admissionRequest.Object.Raw, &pod); err != nil {
		klog.Warningf("failed to unmarshal admissionRequest.Object.Raw to pod, error: %v, admissionRequest: %v", err, admissionRequest)
		return nil, false
	}
	return &pod, true
}

// needInjected 判断 pod 是否需要注入
// pod 的 label 中 label 对应的 value 是需要注入的容器名称，如果不包含则不需要注入。
func needInjected(pod *corev1.Pod, label string) (string, bool) {
	val, exist := pod.Labels[label]
	if !exist {
		return "", false
	}
	return val, true
}

// containerInjected 判断 pod 是否已经包含了指定名称的容器，如果已经存在则不需要注入。
func containerInjected(pod *corev1.Pod, containerName string) bool {
	for _, container := range pod.Spec.InitContainers {
		if container.Name == containerName {
			return true
		}
	}
	for _, container := range pod.Spec.Containers {
		if container.Name == containerName {
			return true
		}
	}
	return false
}

// TODO: responsePatch 根据 containerName 获取 corev1.Container 并生成 AdmissionResponse
func responsePatch(UID types.UID, container *corev1.Container) *admissionv1.AdmissionResponse {
	containerName := container.Name
	// TODO: 通过 containerName 获取对应的 corev1.Container，然后生成 patch
	if container == nil {
		container = &corev1.Container{Name: containerName}
	}

	if ctnPatch, err := json.Marshal([]map[string]any{
		{
			"op":    "add",
			"path":  "/spec/containers/-",
			"value": container,
		},
	}); err == nil {
		patchType := admissionv1.PatchTypeJSONPatch
		return &admissionv1.AdmissionResponse{
			UID:       UID,
			Allowed:   true,
			PatchType: &patchType,
			Patch:     ctnPatch,
		}
	}
	return &admissionv1.AdmissionResponse{
		UID:     UID,
		Allowed: false,
	}
}

func mutateHandler(kv *cache.GeneralCache[*corev1.Container], label string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// admissionReview 的 JSON 反序列化，失败则返回错误响应
		admissionReview := admissionv1.AdmissionReview{}
		if err := c.ShouldBindJSON(&admissionReview); err != nil || admissionReview.Request == nil {
			c.JSON(http.StatusBadRequest, &admissionv1.AdmissionReview{
				Response: &admissionv1.AdmissionResponse{
					Allowed: false,
				},
			})
			return
		}

		// 获取 pod，获取失败后返回错误（非 pod 不应该被发到这个服务端点）
		pod, valid := getPodFromAdmissionReview(&admissionReview)
		if !valid {
			c.JSON(http.StatusBadRequest, &admissionv1.AdmissionReview{
				Response: &admissionv1.AdmissionResponse{
					UID:     admissionReview.Request.UID,
					Allowed: false,
				},
			})
			return
		}

		// 检查 pod 的 label 和 container，判断是否需要注入，如果不需要则直接返回允许的响应
		containerName, need := needInjected(pod, label)
		if !need || containerInjected(pod, containerName) {
			c.JSON(http.StatusOK, &admissionv1.AdmissionReview{
				Response: &admissionv1.AdmissionResponse{
					UID:     admissionReview.Request.UID,
					Allowed: true,
				},
			})
			return
		}

		if container, ok := kv.Get(containerName); ok {
			response := responsePatch(admissionReview.Request.UID, container)
			c.JSON(http.StatusOK, &admissionv1.AdmissionReview{
				Response: response,
			})
			return
		}

		klog.Warningf("container %s not found in cache, cannot inject sidecar", containerName)
		c.JSON(http.StatusOK, &admissionv1.AdmissionReview{
			Response: &admissionv1.AdmissionResponse{
				UID:     admissionReview.Request.UID,
				Allowed: false,
			},
		})
	}
}

func (s *WebhookServer) Aggregation(engine *gin.Engine) {
	// /mutate: 处理 Kubernetes AdmissionReview 请求并返回 sidecar 注入补丁。
	engine.POST("/mutate", mutateHandler(s.kv, s.label))
}

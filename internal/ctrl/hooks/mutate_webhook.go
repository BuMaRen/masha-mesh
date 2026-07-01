package hooks

import (
	"encoding/json"
	"net/http"

	"github.com/BuMaRen/mesh/pkg/cache"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// getPodFromAdmissionReview 的入参 admissionReview 提前检查有效性
func getPodFromAdmissionReview(admissionReview *admissionv1.AdmissionReview) (*corev1.Pod, bool) {
	if admissionReview == nil || admissionReview.Request == nil {
		klog.Warningf("[Webhook] invalid admissionReview: %v", admissionReview)
		return nil, false
	}
	admissionRequest := admissionReview.Request
	pod := corev1.Pod{}
	if err := json.Unmarshal(admissionRequest.Object.Raw, &pod); err != nil {
		klog.Warningf("[Webhook] failed to unmarshal admissionRequest.Object.Raw to pod, error: %v, admissionRequest: %v", err, admissionRequest)
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

type mutateHandler struct {
	kv    *cache.GeneralCache[*corev1.Container]
	label string
}

func responseAdmissionReview(w http.ResponseWriter, statusCode int, body *admissionv1.AdmissionReview) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if body != nil {
		json.NewEncoder(w).Encode(body)
	}
}

func (m *mutateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		responseAdmissionReview(w, http.StatusMethodNotAllowed, nil)
		return
	}
	var admissionReview admissionv1.AdmissionReview
	if err := json.NewDecoder(r.Body).Decode(&admissionReview); err != nil {
		responseAdmissionReview(w, http.StatusBadRequest, nil)
		return
	}
	pod, ok := getPodFromAdmissionReview(&admissionReview)
	if !ok {
		responseAdmissionReview(w, http.StatusBadRequest, nil)
		return
	}
	containerName, needInject := needInjected(pod, m.label)
	if !needInject {
		responseAdmissionReview(w, http.StatusOK, &admissionv1.AdmissionReview{
			Response: &admissionv1.AdmissionResponse{
				UID:     admissionReview.Request.UID,
				Allowed: true,
			},
		})
		return
	}
	if containerInjected(pod, containerName) {
		responseAdmissionReview(w, http.StatusOK, &admissionv1.AdmissionReview{
			Response: &admissionv1.AdmissionResponse{
				UID:     admissionReview.Request.UID,
				Allowed: true,
			},
		})
		return
	}
	container, exist := m.kv.Get(containerName)
	if !exist {
		responseAdmissionReview(w, http.StatusOK, &admissionv1.AdmissionReview{
			Response: &admissionv1.AdmissionResponse{
				UID:     admissionReview.Request.UID,
				Allowed: false,
			},
		})
		return
	}
	responseAdmissionReview(w, http.StatusOK, &admissionv1.AdmissionReview{
		Response: responsePatch(admissionReview.Request.UID, container),
	})
}

func NewMutateHandler(kv *cache.GeneralCache[*corev1.Container], label string) http.Handler {
	return &mutateHandler{
		kv:    kv,
		label: label,
	}
}

var _ http.Handler = (*mutateHandler)(nil)

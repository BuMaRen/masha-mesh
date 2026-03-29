package app

import (
	"encoding/json"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
)

func containerPatch(imageTag, command string) ([]byte, error) {
	cpuLimit, _ := resource.ParseQuantity("100m")
	memLimit, _ := resource.ParseQuantity("128Mi")
	cpuReq, _ := resource.ParseQuantity("50m")
	memReq, _ := resource.ParseQuantity("64Mi")
	ctn := corev1.Container{
		Name:    "sidecar",
		Image:   "hjmasha/mesh-cli:" + imageTag,
		Command: []string{command},
		Resources: corev1.ResourceRequirements{
			Limits:   corev1.ResourceList{"cpu": cpuLimit, "memory": memLimit},
			Requests: corev1.ResourceList{"cpu": cpuReq, "memory": memReq},
		},
	}
	return json.Marshal([]map[string]any{
		{
			"op":    "add",
			"path":  "/spec/containers/-",
			"value": ctn,
		},
	})
}

func getAdmissionResponse(uid, imageTag, command string, allowed bool) (*admissionv1.AdmissionResponse, error) {
	pt := admissionv1.PatchTypeJSONPatch
	patch, err := containerPatch(imageTag, command)
	if err != nil {
		return nil, err
	}
	return &admissionv1.AdmissionResponse{
		UID:       types.UID(uid),
		Allowed:   allowed,
		PatchType: &pt,
		Patch:     patch,
	}, nil
}

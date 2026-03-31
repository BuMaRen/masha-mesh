package logic

import (
	"net/http"

	"encoding/json"

	"github.com/gin-gonic/gin"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/klog/v2"

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
		Image:   imageTag,
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

type admissionReview struct {
	APIVersion string                         `json:"apiVersion"`
	Kind       string                         `json:"kind"`
	Request    *admissionRequest              `json:"request,omitempty"`
	Response   *admissionv1.AdmissionResponse `json:"response,omitempty"`
}

type admissionRequest struct {
	UID       string               `json:"uid"`
	Kind      groupVersionKind     `json:"kind"`
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	Operation string               `json:"operation"`
	UserInfo  admissionRequestUser `json:"userInfo"`
}

type groupVersionKind struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

type admissionRequestUser struct {
	Username string `json:"username"`
}

func Aggregation(engine *gin.Engine, imageTag, command string) {
	engine.POST("/mutate", func(c *gin.Context) {
		var review admissionReview
		if err := c.ShouldBindJSON(&review); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if review.Request == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing admission review request"})
			return
		}

		if review.APIVersion == "" {
			review.APIVersion = "admission.k8s.io/v1"
		}

		response, err := getAdmissionResponse(review.Request.UID, imageTag, command, true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		review.Response = response
		c.JSON(http.StatusOK, review)
		klog.Infof("Handled admission review for %s/%s, operation: %s, user: %s", review.Request.Namespace, review.Request.Name, review.Request.Operation, review.Request.UserInfo.Username)
	})
}

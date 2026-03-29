package app

import (
	"net/http"

	"github.com/gin-gonic/gin"
	admissionv1 "k8s.io/api/admission/v1"
)

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

func Aggregation(engine *gin.Engine) {
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

		// TODO: tag and command should be configurable
		response, err := getAdmissionResponse(review.Request.UID, "v0.1.53", "/app/mesh-cli", true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		review.Response = response
		c.JSON(http.StatusOK, review)
	})
}

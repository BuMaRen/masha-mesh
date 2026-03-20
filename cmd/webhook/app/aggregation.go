package app

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

type admissionReview struct {
	APIVersion string             `json:"apiVersion"`
	Kind       string             `json:"kind"`
	Request    *admissionRequest  `json:"request,omitempty"`
	Response   *admissionResponse `json:"response,omitempty"`
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

type admissionResponse struct {
	UID     string `json:"uid"`
	Allowed bool   `json:"allowed"`
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

		request := review.Request
		klog.Infof(
			"admission request uid=%s operation=%s kind=%s namespace=%s name=%s user=%s",
			request.UID,
			request.Operation,
			request.Kind.Kind,
			request.Namespace,
			request.Name,
			request.UserInfo.Username,
		)

		apiVersion := review.APIVersion
		if apiVersion == "" {
			apiVersion = "admission.k8s.io/v1"
		}

		c.JSON(http.StatusOK, admissionReview{
			APIVersion: apiVersion,
			Kind:       "AdmissionReview",
			Response: &admissionResponse{
				UID:     request.UID,
				Allowed: true,
			},
		})
	})
}

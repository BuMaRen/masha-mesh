package webhook

import (
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestGetPodFromAdmissionReview(tester *testing.T) {
	// getPodFromAdmissionReview(admissionReview *admissionv1.AdmissionReview) (*corev1.Pod, bool)
	tester.Run("nil admissionReview", func(t *testing.T) {
		pod, valid := getPodFromAdmissionReview(nil)
		if pod != nil || valid {
			t.Errorf("Expected nil pod and false valid, got %v and %v", pod, valid)
		}
	})

	tester.Run("nil Request", func(t *testing.T) {
		pod, valid := getPodFromAdmissionReview(&admissionv1.AdmissionReview{
			Request: nil,
		})
		if pod != nil || valid {
			t.Errorf("Expected nil pod and false valid, got %v and %v", pod, valid)
		}
	})

	tester.Run("incorrect Kind", func(t *testing.T) {
		admissionReview := &admissionv1.AdmissionReview{
			Request: &admissionv1.AdmissionRequest{
				Kind: metav1.GroupVersionKind{
					Kind: "Service",
				},
			},
		}
		pod, valid := getPodFromAdmissionReview(admissionReview)
		if pod != nil || valid {
			t.Errorf("Expected nil pod and false valid, got %v and %v", pod, valid)
		}
	})

	tester.Run("invalid Raw JSON", func(t *testing.T) {
		admissionReview := &admissionv1.AdmissionReview{
			Request: &admissionv1.AdmissionRequest{
				Kind: metav1.GroupVersionKind{
					Kind: "Pod",
				},
				Object: runtime.RawExtension{
					Raw: []byte(`invalid json`),
				},
			},
		}
		pod, valid := getPodFromAdmissionReview(admissionReview)
		if pod != nil || valid {
			t.Errorf("Expected nil pod and false valid, got %v and %v", pod, valid)
		}
	})
}

func TestNeedInjected(tester *testing.T) {
	// needInjected(pod *corev1.Pod, label string) (string, bool)
	tester.Run("no labels", func(t *testing.T) {
		pod := &corev1.Pod{}
		val, need := needInjected(pod, "masha.io/injection")
		if need {
			t.Errorf("Expected need false, got true with value %v", val)
		}
	})

	tester.Run("label exists with container name", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"masha.io/injection": "mesh-sidecar",
				},
			},
		}
		val, need := needInjected(pod, "masha.io/injection")
		if !need {
			t.Errorf("Expected need true, got false with value %v", val)
		}
		if val != "mesh-sidecar" {
			t.Errorf("Expected value 'mesh-sidecar', got %v", val)
		}
	})
}

func TestContainerInjected(tester *testing.T) {
	// containerInjected(pod *corev1.Pod, containerName string) bool
	tester.Run("no containers", func(t *testing.T) {
		pod := &corev1.Pod{}
		if containerInjected(pod, "test-container") {
			t.Errorf("Expected container not injected, got injected")
		}
	})

	tester.Run("container exists in initContainers", func(t *testing.T) {
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{
					{Name: "test-container"},
				},
			},
		}
		if !containerInjected(pod, "test-container") {
			t.Errorf("Expected container injected, got not injected")
		}
	})

	tester.Run("container exists in containers", func(t *testing.T) {
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "test-container"},
				},
			},
		}
		if !containerInjected(pod, "test-container") {
			t.Errorf("Expected container injected, got not injected")
		}
	})

	tester.Run("container does not exist", func(t *testing.T) {
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{
					{Name: "other-container"},
				},
				Containers: []corev1.Container{
					{Name: "another-container"},
				},
			},
		}
		if containerInjected(pod, "test-container") {
			t.Errorf("Expected container not injected, got injected")
		}
	})
}

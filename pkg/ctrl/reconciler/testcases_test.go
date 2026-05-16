package reconciler

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func testCase1(nameSpace, labelKey, labelValue string) (appsv1.DeploymentList, appsv1.DeploymentList, *unstructured.Unstructured) {
	return appsv1.DeploymentList{
			Items: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-case-1",
						Namespace: nameSpace,
						UID:       "12345",
						Labels: map[string]string{
							labelKey: labelValue,
						},
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								labelKey: labelValue,
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{labelKey: labelValue},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{Name: "test-container", Image: "nginx:latest"},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-case-2",
						Namespace: nameSpace,
						UID:       "12346",
						Labels: map[string]string{
							labelKey: labelValue,
						},
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								labelKey: labelValue,
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{labelKey: labelValue},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{Name: "test-container", Image: "nginx:latest"},
								},
							},
						},
					},
				},
			},
		}, appsv1.DeploymentList{
			Items: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-case-1",
						Namespace: nameSpace,
						UID:       "12345",
						Labels: map[string]string{
							labelKey: labelValue,
						},
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								labelKey: labelValue,
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{labelKey: labelValue},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{Name: "test-container", Image: "nginx:latest"},
									{Name: "test-sidecar", Image: "sidecar:v1"},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-case-2",
						Namespace: nameSpace,
						UID:       "12346",
						Labels: map[string]string{
							labelKey: labelValue,
						},
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								labelKey: labelValue,
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{labelKey: labelValue},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{Name: "test-container", Image: "nginx:latest"},
									{Name: "test-sidecar", Image: "sidecar:v1"},
								},
							},
						},
					},
				},
			},
		}, &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "mesh.io/v1",
			"kind":       "Container",
			"metadata": map[string]any{
				"name":      "test-sidecar",
				"namespace": nameSpace,
			},
			"spec": map[string]any{
				"name":    "test-sidecar",
				"image":   "sidecar:v1",
				"command": []any{"/app/sidecar"},
			},
		}}
}

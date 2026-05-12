package reconciler

import (
	"context"
	"testing"

	"github.com/BuMaRen/mesh/pkg/ctrl/data"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestOnAdded(t *testing.T) {
	labelKey := "masha.io/injection"
	labelValue := "hjmasha-sidecar-v1"

	inputed, expected, rc := testCase1("default", labelKey, labelValue)

	fakeClient := fake.NewClientset(&inputed)
	c := data.NewContainersCache()
	cr := NewCustomResourcesReconciler(c, fakeClient)
	cr.OnAddedWithContext(context.Background(), labelKey)(rc)
	deps, err := fakeClient.AppsV1().Deployments("default").List(context.Background(), metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key:      labelKey,
				Operator: metav1.LabelSelectorOpExists,
			}},
		}),
	})
	if err != nil {
		t.Fatalf("Failed to list deployments: %v", err)
	}
	if len(deps.Items) != len(expected.Items) {
		t.Fatalf("Expected %d deployments, but got %d", len(expected.Items), len(deps.Items))
	}
	for i, dep := range deps.Items {
		if len(dep.Spec.Template.Spec.Containers) < 2 {
			t.Fatalf("expected at least 2 containers in deployment %s, got %d", dep.Name, len(dep.Spec.Template.Spec.Containers))
		}
		if dep.Spec.Template.Spec.Containers[1].Name != expected.Items[i].Spec.Template.Spec.Containers[1].Name ||
			dep.Spec.Template.Spec.Containers[1].Image != expected.Items[i].Spec.Template.Spec.Containers[1].Image {
			t.Errorf("Expected container: %v, but got: %v", expected.Items[i].Spec.Template.Spec.Containers[1], dep.Spec.Template.Spec.Containers[1])
		}
	}
}

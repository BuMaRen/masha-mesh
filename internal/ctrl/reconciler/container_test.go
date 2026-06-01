package reconciler

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestContainersWithOneRemoved(tester *testing.T) {
	// containersWithOneRemoved(containers []corev1.Container, containerName string) []corev1.Container
	tester.Run("empty containers", func(t *testing.T) {
		containers := []corev1.Container{}
		got := containersWithOneRemoved(containers, "mesh-sidecar")

		if len(got) != 0 {
			t.Fatalf("expected empty result, got len=%d", len(got))
		}
	})

	tester.Run("container exists", func(t *testing.T) {
		containers := []corev1.Container{
			{Name: "app"},
			{Name: "mesh-sidecar"},
			{Name: "logger"},
		}

		got := containersWithOneRemoved(containers, "mesh-sidecar")
		want := []corev1.Container{
			{Name: "app"},
			{Name: "logger"},
		}

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("unexpected result, got=%v want=%v", got, want)
		}
	})

	tester.Run("container exists and len(containers) == 1", func(t *testing.T) {
		containers := []corev1.Container{{Name: "mesh-sidecar"}}

		got := containersWithOneRemoved(containers, "mesh-sidecar")

		if len(got) != 0 {
			t.Fatalf("expected empty result after removing single container, got len=%d", len(got))
		}
	})

	tester.Run("container not exists", func(t *testing.T) {
		containers := []corev1.Container{
			{Name: "app"},
			{Name: "logger"},
		}

		got := containersWithOneRemoved(containers, "mesh-sidecar")

		if !reflect.DeepEqual(got, containers) {
			t.Fatalf("expected containers unchanged, got=%v want=%v", got, containers)
		}
	})
}

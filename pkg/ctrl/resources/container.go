package resources

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

type Container struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ContainerSpec `json:"spec,omitempty"`
}

type ContainerSpec struct {
	Name      string                      `json:"name,omitempty"`
	Image     string                      `json:"image,omitempty"`
	Command   []string                    `json:"command,omitempty"`
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

type ContainerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Container `json:"items"`
}

func ParseContainer(obj any) *Container {
	u := toContainerUnstructured(obj)
	if u == nil {
		return nil
	}

	c := &Container{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, c); err != nil {
		return nil
	}
	return c
}

func toContainerUnstructured(obj any) *unstructured.Unstructured {
	switch t := obj.(type) {
	case *unstructured.Unstructured:
		return t
	case cache.DeletedFinalStateUnknown:
		u, ok := t.Obj.(*unstructured.Unstructured)
		if !ok {
			return nil
		}
		return u
	default:
		return nil
	}
}

func (c *Container) ToCoreV1Container() corev1.Container {
	return corev1.Container{
		Name:      c.Spec.Name,
		Image:     c.Spec.Image,
		Command:   c.Spec.Command,
		Resources: c.Spec.Resources,
	}
}

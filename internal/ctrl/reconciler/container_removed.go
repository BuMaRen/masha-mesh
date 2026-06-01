package reconciler

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

func containersWithOneRemoved(containers []corev1.Container, containerName string) []corev1.Container {
	newContainers := []corev1.Container{}
	for _, container := range containers {
		if container.Name != containerName {
			newContainers = append(newContainers, container)
		}
	}
	return newContainers
}

func deployWithContainerRemoved(deploy *appsv1.Deployment, containerName string) *appsv1.Deployment {
	if deploy == nil {
		klog.Warningf("deploy is nil, cannot remove container %s", containerName)
		return &appsv1.Deployment{}
	}
	spec := &deploy.Spec.Template.Spec
	spec.InitContainers = containersWithOneRemoved(spec.InitContainers, containerName)
	spec.Containers = containersWithOneRemoved(spec.Containers, containerName)
	return deploy
}

func statefulsetWithContainerRemoved(sts *appsv1.StatefulSet, containerName string) *appsv1.StatefulSet {
	if sts == nil {
		klog.Warningf("statefulset is nil, cannot remove container %s", containerName)
		return &appsv1.StatefulSet{}
	}
	spec := &sts.Spec.Template.Spec
	spec.InitContainers = containersWithOneRemoved(spec.InitContainers, containerName)
	spec.Containers = containersWithOneRemoved(spec.Containers, containerName)
	return sts
}

package reconciler

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

func containersWithOneUpdated(containers []corev1.Container, newContainer corev1.Container) []corev1.Container {
	for i, container := range containers {
		if container.Name == newContainer.Name {
			containers[i] = newContainer
			return containers
		}
	}
	// 如果没有找到同名的 container，返回原来的 containers 列表（不做添加）
	return containers
}

func deployWithContainerUpdated(deploy *appsv1.Deployment, newContainer corev1.Container) *appsv1.Deployment {
	if deploy == nil {
		klog.Warningf("deploy is nil, cannot update container %s", newContainer.Name)
		return deploy
	}
	spec := &deploy.Spec.Template.Spec
	spec.InitContainers = containersWithOneUpdated(spec.InitContainers, newContainer)
	spec.Containers = containersWithOneUpdated(spec.Containers, newContainer)
	return deploy
}

func statefulsetWithContainerUpdated(sts *appsv1.StatefulSet, newContainer corev1.Container) *appsv1.StatefulSet {
	if sts == nil {
		klog.Warningf("statefulset is nil, cannot update container %s", newContainer.Name)
		return sts
	}
	spec := &sts.Spec.Template.Spec
	spec.InitContainers = containersWithOneUpdated(spec.InitContainers, newContainer)
	spec.Containers = containersWithOneUpdated(spec.Containers, newContainer)
	return sts
}

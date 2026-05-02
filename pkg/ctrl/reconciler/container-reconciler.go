package reconciler

import (
	"context"
	"fmt"

	"github.com/BuMaRen/mesh/pkg/ctrl/data"
	"github.com/BuMaRen/mesh/pkg/ctrl/resources"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// workQueue 作为一个公共组件从外部注入（webhook也会用到）
type CustomResourcesReconciler struct {
	cache      data.Cache
	kubeClient kubernetes.Interface
}

// reconciler 的职责：
// 1. 监听资源事件，更新 cache
// 2. 将事件推送到 workQueue，待 webhook 消费（webhook 只做 Add 事件）
// 3. 同时更新依赖 container 的其他资源
func NewCustomResourcesReconciler(cache data.Cache, kubeClient kubernetes.Interface) *CustomResourcesReconciler {
	return &CustomResourcesReconciler{
		cache:      cache,
		kubeClient: kubeClient,
	}
}

func getContainer(containerCache data.Cache, containerName string) (*resources.Container, error) {
	obj, existed := containerCache.GetCache(containerName)
	if !existed {
		return nil, fmt.Errorf("container %s not found in cache", containerName)
	}
	ctn := resources.ParseContainer(obj)
	if ctn == nil {
		return nil, fmt.Errorf("failed to parse container %s from cache", containerName)
	}
	return ctn, nil
}

func (r *CustomResourcesReconciler) OnAddedWithContext(ctx context.Context, matchLabels map[string]string) func(obj any) {
	return func(obj any) {
		changed, containerName := r.cache.OnAdded(obj)
		if !changed {
			klog.Warningf("added object is not a valid container, skipping: %v", obj)
			return
		}
		nameSpace := "default"
		ctn := resources.ParseContainer(obj)
		if ctn == nil {
			klog.Warningf("added object is not a valid container, skipping: %v", obj)
		} else {
			nameSpace = ctn.Namespace
		}
		listOpts := metav1.ListOptions{
			LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
				MatchLabels: matchLabels,
			}),
		}

		// workload 存在的时候没有 crd， 所以这里直接查 Deployment/StatefulSet，找到后补上 container
		if deployments, err := r.kubeClient.AppsV1().Deployments(nameSpace).List(ctx, listOpts); err == nil {
			// 这里 deployment 都需要补充 container，如果 container 不存在就补上
			for _, dep := range deployments.Items {
				containers := dep.Spec.Template.Spec.Containers
				skiped := false
				for _, c := range containers {
					if c.Name == containerName {
						skiped = true
						break
					}
				}
				if skiped {
					continue
				}

				containers = append(containers, ctn.ToCoreV1Container())
				dep.Spec.Template.Spec.Containers = containers
				if _, err := r.kubeClient.AppsV1().Deployments(dep.Namespace).Update(ctx, &dep, metav1.UpdateOptions{}); err != nil {
					klog.Errorf("update deployment %s/%s failed while removing container %q: %v", dep.Namespace, dep.Name, containerName, err)
				}
			}
		} else {
			klog.Errorf("list deployment failed while processing added container %q: %v", containerName, err)
		}

		// 更新 StatefulSet：仅移除目标 container，不删除整个 workload
		if statefulSets, err := r.kubeClient.AppsV1().StatefulSets(nameSpace).List(ctx, listOpts); err == nil {
			for _, sts := range statefulSets.Items {
				containers := sts.Spec.Template.Spec.Containers
				skiped := false
				for _, c := range containers {
					if c.Name == containerName {
						skiped = true
						break
					}
				}
				if skiped {
					continue
				}

				sts.Spec.Template.Spec.Containers = containers
				if _, err := r.kubeClient.AppsV1().StatefulSets(sts.Namespace).Update(ctx, &sts, metav1.UpdateOptions{}); err != nil {
					klog.Errorf("update statefulset %s/%s failed while removing container %q: %v", sts.Namespace, sts.Name, containerName, err)
				}
			}
		} else {
			klog.Errorf("list statefulset failed while processing added container %q: %v", containerName, err)
		}
	}
}

func (r *CustomResourcesReconciler) OnUpdatedWithContext(ctx context.Context, matchLabels map[string]string) func(oldObj, newObj any) {
	return func(oldObj, newObj any) {
		changed, containerName := r.cache.OnUpdate(oldObj, newObj)
		if !changed {
			return
		}
		nameSpace := "default"
		ctn := resources.ParseContainer(newObj)
		if ctn == nil {
			klog.Warningf("updated object is not a valid container, skipping: %v", newObj)
		} else {
			nameSpace = ctn.Namespace
		}
		listOpts := metav1.ListOptions{
			LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
				MatchLabels: matchLabels,
			}),
		}

		// ===== 更新 Deployment =====
		if deployments, err := r.kubeClient.AppsV1().Deployments(nameSpace).List(ctx, listOpts); err == nil {
			for _, dep := range deployments.Items {
				// 查找 Pod spec 中的容器，检查是否需要更新
				for i, container := range dep.Spec.Template.Spec.Containers {
					if container.Name != containerName {
						continue
					}
					dep.Spec.Template.Spec.Containers[i].Image = ctn.Spec.Image
					dep.Spec.Template.Spec.Containers[i].Command = ctn.Spec.Command
					dep.Spec.Template.Spec.Containers[i].Resources = ctn.Spec.Resources

					if _, err := r.kubeClient.AppsV1().Deployments(dep.Namespace).Update(ctx, &dep, metav1.UpdateOptions{}); err != nil {
						klog.Errorf("update deployment %s/%s failed: %v", dep.Namespace, dep.Name, err)
						break
					}
				}
			}
		}

		// ===== 更新 StatefulSet =====
		if statefulSets, err := r.kubeClient.AppsV1().StatefulSets(nameSpace).List(ctx, listOpts); err == nil {
			for _, sts := range statefulSets.Items {
				for i, container := range sts.Spec.Template.Spec.Containers {
					if container.Name != containerName {
						continue
					}
					sts.Spec.Template.Spec.Containers[i].Image = ctn.Spec.Image
					sts.Spec.Template.Spec.Containers[i].Command = ctn.Spec.Command
					sts.Spec.Template.Spec.Containers[i].Resources = ctn.Spec.Resources

					if _, err := r.kubeClient.AppsV1().StatefulSets(sts.Namespace).Update(ctx, &sts, metav1.UpdateOptions{}); err != nil {
						klog.Errorf("update statefulset %s/%s failed: %v", sts.Namespace, sts.Name, err)
						break
					}
				}
			}
		}
	}
}

func (r *CustomResourcesReconciler) OnDeletedWithContext(ctx context.Context, matchLabels map[string]string) func(obj any) {
	return func(obj any) {
		changed, containerName, _ := r.cache.OnDelete(obj)
		if !changed {
			return
		}
		nameSpace := "default"
		ctn := resources.ParseContainer(obj)
		if ctn == nil {
			klog.Warningf("deleted object is not a valid container, skipping: %v", obj)
		} else {
			nameSpace = ctn.Namespace
		}
		listOpts := metav1.ListOptions{
			LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
				MatchLabels: matchLabels,
			}),
		}

		// 更新 Deployment：仅移除目标 container，不删除整个 workload
		if deployments, err := r.kubeClient.AppsV1().Deployments(nameSpace).List(ctx, listOpts); err == nil {
			for _, dep := range deployments.Items {
				containers := dep.Spec.Template.Spec.Containers
				newContainers := make([]corev1.Container, 0, len(containers))
				removed := false
				for _, c := range containers {
					if c.Name == containerName {
						removed = true
						continue
					}
					newContainers = append(newContainers, c)
				}

				if !removed {
					continue
				}

				if len(newContainers) == 0 {
					klog.Warningf("skip updating deployment %s/%s: removing container %q would leave pod with no containers", dep.Namespace, dep.Name, containerName)
					continue
				}

				dep.Spec.Template.Spec.Containers = newContainers
				if _, err := r.kubeClient.AppsV1().Deployments(dep.Namespace).Update(ctx, &dep, metav1.UpdateOptions{}); err != nil {
					klog.Errorf("update deployment %s/%s failed while removing container %q: %v", dep.Namespace, dep.Name, containerName, err)
				}
			}
		}

		// 更新 StatefulSet：仅移除目标 container，不删除整个 workload
		if statefulSets, err := r.kubeClient.AppsV1().StatefulSets(nameSpace).List(ctx, listOpts); err == nil {
			for _, sts := range statefulSets.Items {
				containers := sts.Spec.Template.Spec.Containers
				newContainers := make([]corev1.Container, 0, len(containers))
				removed := false
				for _, c := range containers {
					if c.Name == containerName {
						removed = true
						continue
					}
					newContainers = append(newContainers, c)
				}

				if !removed {
					continue
				}

				if len(newContainers) == 0 {
					klog.Warningf("skip updating statefulset %s/%s: removing container %q would leave pod with no containers", sts.Namespace, sts.Name, containerName)
					continue
				}

				sts.Spec.Template.Spec.Containers = newContainers
				if _, err := r.kubeClient.AppsV1().StatefulSets(sts.Namespace).Update(ctx, &sts, metav1.UpdateOptions{}); err != nil {
					klog.Errorf("update statefulset %s/%s failed while removing container %q: %v", sts.Namespace, sts.Name, containerName, err)
				}
			}
		}
	}
}

// func (r *CustomResourcesReconciler) OnDeleted(obj any) {}

// var _ Reconciler = (*CustomResourcesReconciler)(nil)

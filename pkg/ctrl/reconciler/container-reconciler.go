package reconciler

import (
	"context"
	"fmt"

	"github.com/BuMaRen/mesh/pkg/ctrl/data"
	"github.com/BuMaRen/mesh/pkg/ctrl/resources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// workQueue 作为一个公共组件从外部注入（webhook也会用到）
type CustomResourcesReconciler struct {
	cache      data.Cache
	workQueue  workqueue.TypedRateLimitingInterface[string]
	kubeClient kubernetes.Interface
}

// reconciler 的职责：
// 1. 监听资源事件，更新 cache
// 2. 将事件推送到 workQueue，待 webhook 消费（webhook 只做 Add 事件）
// 3. 同时更新依赖 container 的其他资源
func NewCustomResourcesReconciler(cache data.Cache, workQueue workqueue.TypedRateLimitingInterface[string], kubeClient kubernetes.Interface) *CustomResourcesReconciler {
	return &CustomResourcesReconciler{
		cache:      cache,
		workQueue:  workQueue,
		kubeClient: kubeClient,
	}
}

func (r *CustomResourcesReconciler) OnAdded(obj any) {
	if changed, containerName := r.cache.OnAdded(obj); changed {
		// 1. 将 container 变化推送到 workQueue（webhook 消费此队列处理新建资源的注入）
		r.workQueue.Add(containerName)

		// 2. Reconciler 处理：查出所有使用这个 container 的现存资源，更新它们
		// 根据标签查找 Deployment/StatefulSet（例如 "container-name: <containerName>" 或类似标签）
		if err := r.updateExistingResources(containerName); err != nil {
			// 记日志或重试
			_ = err
		}
	}
}

// updateExistingResources 更新所有现存的、使用该 container 的资源
// 直接修改资源的 container spec，导致 rolling update
func (r *CustomResourcesReconciler) updateExistingResources(containerName string) error {
	ctx := context.Background()
	nameSpace := "default"

	// 标签选择器：查找标记为需要注入的资源
	// mesh.io/inject=true: 指示该资源需要 mesh 容器注入
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"mesh.io/inject": "true",
		},
	}
	listOpts := metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&labelSelector),
	}

	// 从 cache 获取最新的 container 定义
	obj, existed := r.cache.GetCache(containerName)
	if !existed {
		return fmt.Errorf("container %s not found in cache", containerName)
	}
	ctn := resources.ParseContainer(obj)
	if ctn == nil {
		return fmt.Errorf("failed to parse container %s from cache", containerName)
	}

	// ===== 更新 Deployment =====
	deployments, err := r.kubeClient.AppsV1().Deployments(nameSpace).List(ctx, listOpts)
	if err != nil {
		return fmt.Errorf("list deployments failed: %w", err)
	}

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
			}
		}

	}

	// ===== 更新 StatefulSet =====
	statefulSets, err := r.kubeClient.AppsV1().StatefulSets(nameSpace).List(ctx, listOpts)
	if err != nil {
		return fmt.Errorf("list statefulsets failed: %w", err)
	}

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
			}
		}
	}

	return nil
}

func (r *CustomResourcesReconciler) OnUpdated(oldObj, newObj any) {
	if changed, containerName := r.cache.OnUpdate(oldObj, newObj); changed {
		r.workQueue.Add(containerName)
	}
}

func (r *CustomResourcesReconciler) OnDeleted(obj any) {
	if changed, containerName, _ := r.cache.OnDelete(obj); changed {
		r.workQueue.Add(containerName)
	}
}

var _ Reconciler = (*CustomResourcesReconciler)(nil)

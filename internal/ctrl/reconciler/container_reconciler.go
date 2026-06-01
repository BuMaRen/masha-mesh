package reconciler

import (
	"context"
	"time"

	"github.com/BuMaRen/mesh/internal/resources"
	"github.com/BuMaRen/mesh/pkg/cache"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var CUSTOM_LABEL = "masha.io/injection"

// workQueue 作为一个公共组件从外部注入（webhook也会用到）
type CustomResourcesReconciler struct {
	cache      cache.Cache
	kubeClient kubernetes.Interface
	label      string
}

// reconciler 的职责：
// 1. 监听资源事件，更新 cache
// 2. 将事件推送到 workQueue，待 webhook 消费（webhook 只做 Add 事件）
// 3. 同时更新依赖 container 的其他资源
func NewCustomResourcesReconciler(cache cache.Cache, label string, kubeClient kubernetes.Interface) *CustomResourcesReconciler {
	return &CustomResourcesReconciler{
		cache:      cache,
		kubeClient: kubeClient,
		label:      label,
	}
}

func (r *CustomResourcesReconciler) listDeployments(ctx context.Context, nameSpace string) (*appsv1.DeploymentList, error) {
	listOpts := metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: r.label, Operator: metav1.LabelSelectorOpExists},
			},
		}),
	}
	return r.kubeClient.AppsV1().Deployments(nameSpace).List(ctx, listOpts)
}

func (r *CustomResourcesReconciler) listStatefulSets(ctx context.Context, nameSpace string) (*appsv1.StatefulSetList, error) {
	listOpts := metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: r.label, Operator: metav1.LabelSelectorOpExists},
			},
		}),
	}
	return r.kubeClient.AppsV1().StatefulSets(nameSpace).List(ctx, listOpts)
}

// TODO: 添加的时候只做缓存，注入交给 webhook
func (r *CustomResourcesReconciler) OnAdded(obj any) {
	changed, _ := r.cache.OnAdded(obj)
	if !changed {
		klog.Warningf("added object is not a valid container, skipping: %v", obj)
		return
	}
}

func (r *CustomResourcesReconciler) OnUpdated(oldObj, newObj any) {
	if changed, _ := r.cache.OnUpdate(oldObj, newObj); !changed {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	customContainer := resources.ParseContainer(newObj)
	if customContainer == nil {
		klog.Errorf("updated object is not a valid container, skipping: %v", newObj)
		return
	}
	nameSpace := customContainer.Namespace

	// ===== 更新 Deployment =====
	if deployments, err := r.listDeployments(ctx, nameSpace); err == nil {
		for _, dep := range deployments.Items {
			newDeploy := deployWithContainerUpdated(&dep, customContainer.ToCoreV1Container())
			if _, err := r.kubeClient.AppsV1().Deployments(dep.Namespace).Update(ctx, newDeploy, metav1.UpdateOptions{}); err != nil {
				klog.Errorf("update deployment %s/%s failed: %v", dep.Namespace, dep.Name, err)
			}
		}
	}

	// ===== 更新 StatefulSet =====
	if statefulSets, err := r.listStatefulSets(ctx, nameSpace); err == nil {
		for _, sts := range statefulSets.Items {
			newSts := statefulsetWithContainerUpdated(&sts, customContainer.ToCoreV1Container())
			if _, err := r.kubeClient.AppsV1().StatefulSets(sts.Namespace).Update(ctx, newSts, metav1.UpdateOptions{}); err != nil {
				klog.Errorf("update statefulset %s/%s failed: %v", sts.Namespace, sts.Name, err)
			}
		}
	}
}

func (r *CustomResourcesReconciler) OnDeleted(obj any) {
	changed, containerName, _ := r.cache.OnDelete(obj)
	if !changed {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	nameSpace := resources.Namespace(obj)

	// 更新 Deployment：仅移除目标 container，不删除整个 workload
	if deployments, err := r.listDeployments(ctx, nameSpace); err == nil {
		for _, dep := range deployments.Items {
			newDeploy := deployWithContainerRemoved(&dep, containerName)
			if _, err := r.kubeClient.AppsV1().Deployments(dep.Namespace).Update(ctx, newDeploy, metav1.UpdateOptions{}); err != nil {
				klog.Errorf("update deployment %s/%s failed while removing container %q: %v", dep.Namespace, dep.Name, containerName, err)
			}
		}
	}

	// 更新 StatefulSet：仅移除目标 container，不删除整个 workload
	if statefulSets, err := r.listStatefulSets(ctx, nameSpace); err == nil {
		for _, sts := range statefulSets.Items {
			newSts := statefulsetWithContainerRemoved(&sts, containerName)
			if _, err := r.kubeClient.AppsV1().StatefulSets(sts.Namespace).Update(ctx, newSts, metav1.UpdateOptions{}); err != nil {
				klog.Errorf("update statefulset %s/%s failed while removing container %q: %v", sts.Namespace, sts.Name, containerName, err)
			}
		}
	}
}

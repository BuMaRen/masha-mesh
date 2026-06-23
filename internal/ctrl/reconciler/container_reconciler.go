package reconciler

import (
	"context"
	"time"

	"github.com/BuMaRen/mesh/internal/resources"
	"github.com/BuMaRen/mesh/pkg/cache"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// workQueue 作为一个公共组件从外部注入（webhook也会用到）
type CustomResourcesReconciler struct {
	kv         *cache.GeneralCache[*corev1.Container]
	kubeClient kubernetes.Interface
	label      string
}

// reconciler 的职责：
// 1. 监听资源事件，更新 cache
// 2. 将事件推送到 workQueue，待 webhook 消费（webhook 只做 Add 事件）
// 3. 同时更新依赖 container 的其他资源
func NewCustomResourcesReconciler(kv *cache.GeneralCache[*corev1.Container], label string, kubeClient kubernetes.Interface) *CustomResourcesReconciler {
	return &CustomResourcesReconciler{
		kubeClient: kubeClient,
		label:      label,
		kv:         kv,
	}
}

func (r *CustomResourcesReconciler) listDeployments(nameSpace string) (*appsv1.DeploymentList, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	listOpts := metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: r.label, Operator: metav1.LabelSelectorOpExists},
			},
		}),
	}
	return r.kubeClient.AppsV1().Deployments(nameSpace).List(ctx, listOpts)
}

func (r *CustomResourcesReconciler) listStatefulSets(nameSpace string) (*appsv1.StatefulSetList, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
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
	customContainer := resources.ParseContainer(obj)
	if customContainer == nil {
		klog.Warningf("[Reconciler] added object is not a valid container, skipping: %v", obj)
		return
	}
	coreContainer := customContainer.ToCoreV1Container()
	_, ok := r.kv.Add(coreContainer.Name, &coreContainer)
	klog.Infof("[Reconciler] container %s added to cache, already existed: %v", coreContainer.Name, !ok)
}

// TODO：如何面对 containerName 变化的情况？极低价值
func (r *CustomResourcesReconciler) OnUpdated(oldObj, newObj any) {
	newContainer := resources.ParseContainer(newObj)
	if newContainer == nil {
		klog.Warningf("[Reconciler] updated object is not a valid container, skipping: %v", newObj)
		return
	}
	coreContainer := newContainer.ToCoreV1Container()
	if _, ok := r.kv.Update(coreContainer.Name, &coreContainer); !ok {
		return
	}
	nameSpace := newContainer.Namespace

	// ===== 更新 Deployment =====
	if deployments, err := r.listDeployments(nameSpace); err == nil {
		for _, dep := range deployments.Items {
			newDeploy := deployWithContainerUpdated(&dep, coreContainer)
			if err := updateDeployment(r.kubeClient, newDeploy); err != nil {
				klog.Errorf("[Reconciler] update deployment %s/%s failed: %v", dep.Namespace, dep.Name, err)
			}
		}
	}

	// ===== 更新 StatefulSet =====
	if statefulSets, err := r.listStatefulSets(nameSpace); err == nil {
		for _, sts := range statefulSets.Items {
			newSts := statefulsetWithContainerUpdated(&sts, coreContainer)
			if err := updateStatefulSet(r.kubeClient, newSts); err != nil {
				klog.Errorf("[Reconciler] update statefulset %s/%s failed: %v", sts.Namespace, sts.Name, err)
			}
		}
	}
}

func (r *CustomResourcesReconciler) OnDeleted(obj any) {
	container := resources.ParseContainer(obj)
	if container == nil {
		klog.Warningf("[Reconciler] deleted object is not a valid container, skipping: %v", obj)
		return
	}
	coreContainer := container.ToCoreV1Container()
	if _, ok := r.kv.Delete(coreContainer.Name); !ok {
		return
	}
	nameSpace := container.Namespace
	containerName := container.Spec.Name

	// 更新 Deployment：仅移除目标 container，不删除整个 workload
	if deployments, err := r.listDeployments(nameSpace); err == nil {
		for _, dep := range deployments.Items {
			newDeploy := deployWithContainerRemoved(&dep, containerName)
			if err := updateDeployment(r.kubeClient, newDeploy); err != nil {
				klog.Errorf("[Reconciler] update deployment %s/%s failed while removing container %q: %v", dep.Namespace, dep.Name, containerName, err)
			}
		}
	}

	// 更新 StatefulSet：仅移除目标 container，不删除整个 workload
	if statefulSets, err := r.listStatefulSets(nameSpace); err == nil {
		for _, sts := range statefulSets.Items {
			newSts := statefulsetWithContainerRemoved(&sts, containerName)
			if err := updateStatefulSet(r.kubeClient, newSts); err != nil {
				klog.Errorf("[Reconciler] update statefulset %s/%s failed while removing container %q: %v", sts.Namespace, sts.Name, containerName, err)
			}
		}
	}
}

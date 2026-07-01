package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/BuMaRen/mesh/pkg/cache"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// CRDWorker holds the workqueue and cache, and drives the reconcile loop for CRD events.
// Use Handlers() to obtain informer callbacks that feed events into this worker.
type CRDWorker struct {
	queue      workqueue.TypedRateLimitingInterface[string]
	cache      *cache.GeneralCache[*corev1.Container]
	kubeClient kubernetes.Interface
	label      string
	maxRetries int
}

func NewCRDWorker(
	c *cache.GeneralCache[*corev1.Container],
	label string,
	kubeClient kubernetes.Interface,
	maxRetries int,
) *CRDWorker {
	rateLimiter := workqueue.NewTypedItemExponentialFailureRateLimiter[string](1*time.Second, 60*time.Second)
	return &CRDWorker{
		queue:      workqueue.NewTypedRateLimitingQueue(rateLimiter),
		cache:      c,
		kubeClient: kubeClient,
		label:      label,
		maxRetries: maxRetries,
	}
}

// Handlers returns a CRDHandlers that shares this worker's queue and cache.
// CRDHandlers and CRDWorker are peers: neither owns the other.
func (w *CRDWorker) Handlers() *CRDHandlers {
	return &CRDHandlers{
		cache: w.cache,
		queue: w.queue,
	}
}

// Run starts concurrency worker goroutines and blocks until ctx is cancelled.
func (w *CRDWorker) Run(ctx context.Context, concurrency int) {
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.runWorker()
		}()
	}
	<-ctx.Done()
	w.queue.ShutDown()
	wg.Wait()
}

func (w *CRDWorker) runWorker() {
	for {
		key, shutdown := w.queue.Get()
		if shutdown {
			return
		}
		w.process(key)
	}
}

func (w *CRDWorker) process(key string) {
	defer w.queue.Done(key)

	event, err := parseEventKey(key)
	if err != nil {
		klog.Errorf("[CRDWorker] invalid key %q, dropping: %v", key, err)
		w.queue.Forget(key)
		return
	}

	if err := w.processEvent(event); err != nil {
		if w.shouldRetry(key) {
			klog.Errorf("[CRDWorker] error processing %q, enqueue with exponential backoff: %v", key, err)
			w.queue.AddRateLimited(key)
			return
		}
		klog.Errorf("[CRDWorker] dropping %q after %d retries: %v", key, w.maxRetries, err)
	}
	w.queue.Forget(key)
}

func (w *CRDWorker) shouldRetry(key string) bool {
	if w.maxRetries < 0 {
		return true
	}
	return w.queue.NumRequeues(key) < w.maxRetries
}

func (w *CRDWorker) processEvent(event *CRDWorkerEvent) error {
	switch event.Type {
	case EventTypeAdd:
		// Cache is already updated by the handler; webhook handles sidecar injection.
		return nil
	case EventTypeUpdate:
		return w.reconcileUpdate(event.ContainerName, event.Namespace)
	case EventTypeDelete:
		return w.reconcileDelete(event.ContainerName, event.Namespace)
	default:
		klog.Warningf("[CRDWorker] unknown event type %q for %s/%s",
			event.Type, event.Namespace, event.ContainerName)
		return nil
	}
}

func (w *CRDWorker) reconcileUpdate(containerName, namespace string) error {
	container, ok := w.cache.Get(containerName)
	if !ok {
		klog.Infof("[CRDWorker] container %s not found in cache during Update reconcile, skipping", containerName)
		return nil
	}

	var errs []error

	deployments, err := w.listDeployments(namespace)
	if err != nil {
		return fmt.Errorf("list deployments in namespace %s: %w", namespace, err)
	}
	for i := range deployments.Items {
		updated := deployWithContainerUpdated(&deployments.Items[i], *container)
		if err := w.updateDeployment(updated); err != nil {
			errs = append(errs, fmt.Errorf("update deployment %s/%s: %w", updated.Namespace, updated.Name, err))
		}
	}

	statefulSets, err := w.listStatefulSets(namespace)
	if err != nil {
		return fmt.Errorf("list statefulsets in namespace %s: %w", namespace, err)
	}
	for i := range statefulSets.Items {
		updated := statefulsetWithContainerUpdated(&statefulSets.Items[i], *container)
		if err := w.updateStatefulSet(updated); err != nil {
			errs = append(errs, fmt.Errorf("update statefulset %s/%s: %w", updated.Namespace, updated.Name, err))
		}
	}

	return errors.Join(errs...)
}

func (w *CRDWorker) reconcileDelete(containerName, namespace string) error {
	var errs []error

	deployments, err := w.listDeployments(namespace)
	if err != nil {
		return fmt.Errorf("list deployments in namespace %s: %w", namespace, err)
	}
	for i := range deployments.Items {
		updated := deployWithContainerRemoved(&deployments.Items[i], containerName)
		if err := w.updateDeployment(updated); err != nil {
			errs = append(errs, fmt.Errorf("update deployment %s/%s: %w", updated.Namespace, updated.Name, err))
		}
	}

	statefulSets, err := w.listStatefulSets(namespace)
	if err != nil {
		return fmt.Errorf("list statefulsets in namespace %s: %w", namespace, err)
	}
	for i := range statefulSets.Items {
		updated := statefulsetWithContainerRemoved(&statefulSets.Items[i], containerName)
		if err := w.updateStatefulSet(updated); err != nil {
			errs = append(errs, fmt.Errorf("update statefulset %s/%s: %w", updated.Namespace, updated.Name, err))
		}
	}

	return errors.Join(errs...)
}

// --- Kubernetes API helpers ---

func (w *CRDWorker) listDeployments(namespace string) (*appsv1.DeploymentList, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return w.kubeClient.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: w.label, Operator: metav1.LabelSelectorOpExists},
			},
		}),
	})
}

func (w *CRDWorker) listStatefulSets(namespace string) (*appsv1.StatefulSetList, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return w.kubeClient.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: w.label, Operator: metav1.LabelSelectorOpExists},
			},
		}),
	})
}

func (w *CRDWorker) updateDeployment(dep *appsv1.Deployment) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := w.kubeClient.AppsV1().Deployments(dep.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}

func (w *CRDWorker) updateStatefulSet(sts *appsv1.StatefulSet) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := w.kubeClient.AppsV1().StatefulSets(sts.Namespace).Update(ctx, sts, metav1.UpdateOptions{})
	return err
}

// --- Container manipulation helpers ---

func containersWithOneUpdated(containers []corev1.Container, newContainer corev1.Container) []corev1.Container {
	for i, c := range containers {
		if c.Name == newContainer.Name {
			containers[i] = newContainer
			return containers
		}
	}
	return containers
}

func deployWithContainerUpdated(deploy *appsv1.Deployment, newContainer corev1.Container) *appsv1.Deployment {
	spec := &deploy.Spec.Template.Spec
	spec.InitContainers = containersWithOneUpdated(spec.InitContainers, newContainer)
	spec.Containers = containersWithOneUpdated(spec.Containers, newContainer)
	return deploy
}

func statefulsetWithContainerUpdated(sts *appsv1.StatefulSet, newContainer corev1.Container) *appsv1.StatefulSet {
	spec := &sts.Spec.Template.Spec
	spec.InitContainers = containersWithOneUpdated(spec.InitContainers, newContainer)
	spec.Containers = containersWithOneUpdated(spec.Containers, newContainer)
	return sts
}

func containersWithOneRemoved(containers []corev1.Container, containerName string) []corev1.Container {
	result := make([]corev1.Container, 0, len(containers))
	for _, c := range containers {
		if c.Name != containerName {
			result = append(result, c)
		}
	}
	return result
}

func deployWithContainerRemoved(deploy *appsv1.Deployment, containerName string) *appsv1.Deployment {
	spec := &deploy.Spec.Template.Spec
	spec.InitContainers = containersWithOneRemoved(spec.InitContainers, containerName)
	spec.Containers = containersWithOneRemoved(spec.Containers, containerName)
	return deploy
}

func statefulsetWithContainerRemoved(sts *appsv1.StatefulSet, containerName string) *appsv1.StatefulSet {
	spec := &sts.Spec.Template.Spec
	spec.InitContainers = containersWithOneRemoved(spec.InitContainers, containerName)
	spec.Containers = containersWithOneRemoved(spec.Containers, containerName)
	return sts
}

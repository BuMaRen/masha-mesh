package worker

import (
	"github.com/BuMaRen/mesh/internal/resources"
	"github.com/BuMaRen/mesh/pkg/cache"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// CRDHandlers implements informer event callbacks.
// It is a peer of CRDWorker: both share the same cache and queue as dependencies.
// Each callback updates the shared cache and enqueues a CRDWorkerEvent for the worker.
type CRDHandlers struct {
	cache *cache.GeneralCache[*corev1.Container]
	queue workqueue.TypedRateLimitingInterface[string]
}

func (h *CRDHandlers) enqueue(event *CRDWorkerEvent) {
	// Initial delivery should be immediate; backoff is applied only when worker requeues after errors.
	h.queue.Add(event.key())
}

func (h *CRDHandlers) OnAdded(obj any) {
	container := resources.ParseContainer(obj)
	if container == nil {
		klog.Warningf("[CRDHandlers] added object is not a valid container: %v", obj)
		return
	}
	coreContainer := container.ToCoreV1Container()
	_, existed := h.cache.Add(coreContainer.Name, &coreContainer)
	klog.Infof("[CRDHandlers] container %s added to cache, already existed: %v", coreContainer.Name, existed)
	h.enqueue(&CRDWorkerEvent{
		Type:          EventTypeAdd,
		ContainerName: coreContainer.Name,
		Namespace:     container.Namespace,
	})
}

func (h *CRDHandlers) OnUpdated(_, newObj any) {
	container := resources.ParseContainer(newObj)
	if container == nil {
		klog.Warningf("[CRDHandlers] updated object is not a valid container: %v", newObj)
		return
	}
	coreContainer := container.ToCoreV1Container()
	if _, ok := h.cache.Update(coreContainer.Name, &coreContainer); !ok {
		klog.Warningf("[CRDHandlers] container %s not in cache, skipping update enqueue", coreContainer.Name)
		return
	}
	h.enqueue(&CRDWorkerEvent{
		Type:          EventTypeUpdate,
		ContainerName: coreContainer.Name,
		Namespace:     container.Namespace,
	})
}

func (h *CRDHandlers) OnDeleted(obj any) {
	container := resources.ParseContainer(obj)
	if container == nil {
		klog.Warningf("[CRDHandlers] deleted object is not a valid container: %v", obj)
		return
	}
	coreContainer := container.ToCoreV1Container()
	if _, ok := h.cache.Delete(coreContainer.Name); !ok {
		klog.Warningf("[CRDHandlers] container %s not in cache, skipping delete enqueue", coreContainer.Name)
		return
	}
	h.enqueue(&CRDWorkerEvent{
		Type:          EventTypeDelete,
		ContainerName: coreContainer.Name,
		Namespace:     container.Namespace,
	})
}

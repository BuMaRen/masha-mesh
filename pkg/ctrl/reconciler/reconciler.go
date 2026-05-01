package reconciler

import (
	"github.com/BuMaRen/mesh/pkg/api/mesh"
	"github.com/BuMaRen/mesh/pkg/ctrl/data"
	"github.com/BuMaRen/mesh/pkg/ctrl/distributer"
)

type EndpointSliceReconciler struct {
	cache       data.Cache
	distributer distributer.Distributer
}

func NewEndpointSliceReconciler(cache data.Cache, distributer distributer.Distributer) Reconciler {
	return &EndpointSliceReconciler{cache: cache, distributer: distributer}
}

// TODO: only pushing all notifications for now; switch to incremental updates later.
func (r *EndpointSliceReconciler) OnAdded(obj any) {
	if changed, svcName := r.cache.OnAdded(obj); changed {
		if mergedEs, ok := r.cache.GetMerged(svcName); ok {
			r.distributer.Publish(svcName, mesh.OpType_ADDED, mergedEs)
		}
	}
}

func (r *EndpointSliceReconciler) OnUpdated(oldObj, newObj any) {
	if changed, svcName := r.cache.OnUpdate(oldObj, newObj); changed {
		if mergedEs, ok := r.cache.GetMerged(svcName); ok {
			r.distributer.Publish(svcName, mesh.OpType_MODIFIED, mergedEs)
		}
	}
}

func (r *EndpointSliceReconciler) OnDeleted(obj any) {
	if changed, svcName, deleteAll := r.cache.OnDelete(obj); changed {
		mergedEs, ok := r.cache.GetMerged(svcName)
		if deleteAll {
			r.distributer.Publish(svcName, mesh.OpType_DELETED, mergedEs)
			return
		}
		if ok {
			r.distributer.Publish(svcName, mesh.OpType_MODIFIED, mergedEs)
		}
	}
}

var _ Reconciler = (*EndpointSliceReconciler)(nil)

package ctrl

import (
	"context"
	"sync"

	"github.com/BuMaRen/mesh/pkg/ctrl/data"
	"github.com/BuMaRen/mesh/pkg/ctrl/grpcserver"
	rc "github.com/BuMaRen/mesh/pkg/ctrl/reconciler"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func StartUp(ctx context.Context, opts *Options) {
	grpcSvr := grpcserver.NewGrpcServer()
	distributer := grpcSvr.Distributer()
	localCache := data.NewEndpointSliceCache()
	reconciler := rc.NewEndpointSliceReconciler(localCache, distributer)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		WatchEndpointSlice(ctx, cache.ResourceEventHandlerFuncs{
			AddFunc:    reconciler.OnAdded,
			UpdateFunc: reconciler.OnUpdated,
			DeleteFunc: reconciler.OnDeleted,
		})
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := grpcSvr.ListenAndServe(ctx, opts.GrpcOptions()); err != nil {
			klog.Errorf("Failed to start gRPC server: %v", err)
		}
	}()

	wg.Wait()
}

package ctrl

import (
	"context"
	"sync"

	"github.com/BuMaRen/mesh/pkg/ctrl/data"
	"github.com/BuMaRen/mesh/pkg/ctrl/grpcserver"
	rc "github.com/BuMaRen/mesh/pkg/ctrl/reconciler"
	"github.com/BuMaRen/mesh/pkg/ctrl/utils"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func StartUp(ctx context.Context, opts *Options) {
	grpcSvr := grpcserver.NewGrpcServer()
	distributer := grpcSvr.Distributer()

	containerCache := data.NewContainersCache()
	epsCache := data.NewEndpointSliceCache()

	k8sClient := utils.NewKubernetesClientOrDie()
	dynamicClient := utils.NewDynamicClientOrDie()

	endpointsliceReconciler := rc.NewEndpointSliceReconciler(epsCache, distributer)
	customResourcesReconciler := rc.NewCustomResourcesReconciler(containerCache, k8sClient)

	// 监听 endpointSlice，用于路由
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		WatchEndpointSlice(ctx, k8sClient, cache.ResourceEventHandlerFuncs{
			AddFunc:    endpointsliceReconciler.OnAdded,
			UpdateFunc: endpointsliceReconciler.OnUpdated,
			DeleteFunc: endpointsliceReconciler.OnDeleted,
		})
	}()

	// 监听用户自定义资源，用于注入
	wg.Add(1)
	go func() {
		defer wg.Done()
		matchLabels := map[string]string{
			"masha.io/injection": "true",
		}
		WatchCRD(ctx, dynamicClient, schema.GroupVersionResource{
			Group:    opts.gvrGroup,
			Version:  opts.gvrVersion,
			Resource: opts.gvrResource,
		}, cache.ResourceEventHandlerFuncs{
			AddFunc:    customResourcesReconciler.OnAddedWithContext(ctx, matchLabels),
			UpdateFunc: customResourcesReconciler.OnUpdatedWithContext(ctx, matchLabels),
			DeleteFunc: customResourcesReconciler.OnDeletedWithContext(ctx, matchLabels),
		})
	}()

	// 启动 gRPC 服务器，与 sidecar 交互
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := grpcSvr.ListenAndServe(ctx, opts.GrpcOptions()); err != nil {
			klog.Errorf("Failed to start gRPC server: %v", err)
		}
	}()

	wg.Wait()
}

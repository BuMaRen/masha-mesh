package ctrl

import (
	"context"
	"sync"

	"github.com/BuMaRen/mesh/pkg/ctrl/data"
	"github.com/BuMaRen/mesh/pkg/ctrl/grpcserver"
	"github.com/BuMaRen/mesh/pkg/ctrl/metrics"
	rc "github.com/BuMaRen/mesh/pkg/ctrl/reconciler"
	"github.com/BuMaRen/mesh/pkg/ctrl/utils"
	"github.com/BuMaRen/mesh/pkg/ctrl/webhook"
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

	webhook.NewWebhookServer(containerCache)
	endpointsliceReconciler := rc.NewEndpointSliceReconciler(epsCache, distributer)
	customResourcesReconciler := rc.NewCustomResourcesReconciler(containerCache, k8sClient)

	wg := sync.WaitGroup{}
	// 启动 metrics 服务器
	metrics.MustRegister()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := metrics.RunMetricsServer(ctx, opts.MetricsOptions()); err != nil {
			klog.Errorf("Failed to start metrics server: %v", err)
		}
	}()

	// 监听 endpointSlice，用于路由。依赖 grpc，grpc 启动前 sidecars 为空， publish 无法起到实际作用。
	wg.Add(1)
	go func() {
		defer wg.Done()
		WatchEndpointSlice(ctx, k8sClient, cache.ResourceEventHandlerFuncs{
			AddFunc:    endpointsliceReconciler.OnAdded,
			UpdateFunc: endpointsliceReconciler.OnUpdated,
			DeleteFunc: endpointsliceReconciler.OnDeleted,
		})
	}()

	// 监听自定义的资源，只依赖 kubernetes 原生的资源。
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
		if err := grpcSvr.ListenAndServe(ctx, opts.CtrlOptions()); err != nil {
			klog.Errorf("Failed to start gRPC server: %v", err)
		}
	}()

	wg.Wait()
}

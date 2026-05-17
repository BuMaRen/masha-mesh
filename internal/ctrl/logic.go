package ctrl

import (
	"context"
	"sync"

	"github.com/BuMaRen/mesh/internal/ctrl/grpcserver"
	rc "github.com/BuMaRen/mesh/internal/ctrl/reconciler"
	"github.com/BuMaRen/mesh/internal/ctrl/webhook"
	"github.com/BuMaRen/mesh/internal/resources"
	"github.com/BuMaRen/mesh/pkg/metrics"
	"github.com/BuMaRen/mesh/pkg/utils"

	"k8s.io/apimachinery/pkg/runtime/schema"
	k8Cache "k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func StartUp(ctx context.Context, opts *Options) {
	grpcSvr := grpcserver.NewGrpcServer()
	distributer := grpcSvr.Distributer()

	containerCache := resources.NewContainersCache()
	epsCache := resources.NewEndpointSliceCache()

	k8sClient := utils.NewKubernetesClientOrDie()
	dynamicClient := utils.NewDynamicClientOrDie()

	webhookServer := webhook.NewWebhookServer(containerCache, webhook.WithInjectionLabel(opts.label))
	endpointsliceReconciler := rc.NewEndpointSliceReconciler(epsCache, distributer)
	customResourcesReconciler := rc.NewCustomResourcesReconciler(containerCache, opts.label, k8sClient)

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

	// 启动 webhook 服务器，监听自定义资源 container 的变化
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := webhookServer.Run(ctx, opts.WebhookOptions()); err != nil {
			klog.Errorf("Failed to start webhook server: %v", err)
		}
	}()

	// 监听 endpointSlice，用于路由。依赖 grpc，grpc 启动前 sidecars 为空， publish 无法起到实际作用。
	wg.Add(1)
	go func() {
		defer wg.Done()
		WatchEndpointSlice(ctx, k8sClient, k8Cache.ResourceEventHandlerFuncs{
			AddFunc:    endpointsliceReconciler.OnAdded,
			UpdateFunc: endpointsliceReconciler.OnUpdated,
			DeleteFunc: endpointsliceReconciler.OnDeleted,
		})
	}()

	// 监听自定义的资源，只依赖 kubernetes 原生的资源。
	wg.Add(1)
	go func() {
		defer wg.Done()
		WatchCRD(ctx, dynamicClient, schema.GroupVersionResource{
			Group:    opts.gvrGroup,
			Version:  opts.gvrVersion,
			Resource: opts.gvrResource,
		}, k8Cache.ResourceEventHandlerFuncs{
			AddFunc:    customResourcesReconciler.OnAdded,
			UpdateFunc: customResourcesReconciler.OnUpdated,
			DeleteFunc: customResourcesReconciler.OnDeleted,
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

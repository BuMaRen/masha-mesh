package ctrl

import (
	"context"
	"sync"
	"time"

	"github.com/BuMaRen/mesh/internal/ctrl/grpcserver"
	"github.com/BuMaRen/mesh/internal/ctrl/hooks"
	rc "github.com/BuMaRen/mesh/internal/ctrl/reconciler"
	"github.com/BuMaRen/mesh/internal/ctrl/worker"
	"github.com/BuMaRen/mesh/internal/resources"
	"github.com/BuMaRen/mesh/pkg/cache"
	"github.com/BuMaRen/mesh/pkg/metrics"
	"github.com/BuMaRen/mesh/pkg/utils"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8Cache "k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func StartUp(rootContext context.Context, opts *StartUpOptions) {
	httpSvr := NewHttpsServer(opts)
	grpcSvr := grpcserver.NewGrpcServer()
	distributer := grpcSvr.Distributer()

	containerCache := cache.NewGeneralCache[*corev1.Container]()
	epsCache := resources.NewEndpointSliceCache()

	k8sClient := utils.NewKubernetesClientOrDie()
	dynamicClient := utils.NewDynamicClientOrDie()

	endpointsliceReconciler := rc.NewEndpointSliceReconciler(epsCache, distributer)
	crdWorker := worker.NewCRDWorker(containerCache, opts.label, k8sClient, opts.crdWorkerMaxRetries)
	crdHandlers := crdWorker.Handlers()

	stopCh := make(chan struct{})
	ctx, cancel := context.WithCancel(rootContext)
	defer cancel()
	wg := sync.WaitGroup{}

	metrics.MustRegister()
	httpSvr.RegisterHandler("/prometheus", promhttp.Handler())
	// 处理 preStop 请求，通知 sidecar 进行预停止准备
	httpSvr.RegisterHandler("/preStop", hooks.NewPreStopHandler(stopCh))
	// 处理 mutate 请求，进行注入逻辑
	httpSvr.RegisterHandler("/mutate", hooks.NewMutateHandler(containerCache, opts.label))
	// readyz：聚合 gRPC 服务器 + kube-client 的就绪状态供 Kubernetes readinessProbe 调用
	httpSvr.RegisterHandler("/readyz", hooks.NewReadyzHandler(
		5*time.Second,
		hooks.NewGrpcServerChecker(grpcSvr.IsReady),
		hooks.NewKubeClientChecker(k8sClient),
		hooks.NewDynamicClientChecker(dynamicClient, schema.GroupVersionResource{
			Group:    opts.gvrGroup,
			Version:  opts.gvrVersion,
			Resource: opts.gvrResource,
		}),
		// note: Prometheus 使用 pull 模式，因此这里直接返回 true
		hooks.NewMetricsServerChecker(func() bool { return true }),
	))
	httpSvr.RegisterHandler("/healthz", hooks.NewHealthzHandler(
		5*time.Second,
		hooks.NewDefaultHealthChecker("default-healthz"),
	))
	wg.Add(1)
	go func() {
		defer wg.Done()
		httpSvr.ServeTLS(ctx, stopCh)
		cancel() // 关闭 HTTP 服务器后取消上下文，通知其他 goroutine 进行清理和退出
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
		crdWorker.Run(ctx, 1)
	}()

	// 监听 CRD 事件并交给 CRDHandlers 更新缓存和入队。
	wg.Add(1)
	go func() {
		defer wg.Done()
		WatchCRD(ctx, dynamicClient, schema.GroupVersionResource{
			Group:    opts.gvrGroup,
			Version:  opts.gvrVersion,
			Resource: opts.gvrResource,
		}, k8Cache.ResourceEventHandlerFuncs{
			AddFunc:    crdHandlers.OnAdded,
			UpdateFunc: crdHandlers.OnUpdated,
			DeleteFunc: crdHandlers.OnDeleted,
		})
	}()

	// 启动 gRPC 服务器，与 sidecar 交互
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := grpcSvr.ListenAndServe(ctx, opts.GrpcOptions()); err != nil {
			klog.Errorf("[GrpcServer] failed to start: %v", err)
		}
	}()

	wg.Wait()
}

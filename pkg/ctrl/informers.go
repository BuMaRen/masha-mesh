package ctrl

import (
	"context"

	"github.com/BuMaRen/mesh/pkg/ctrl/utils"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// WatchCRD 阻塞监听指定 CRD 的变化，并通过事件处理函数 fns 进行处理
func WatchCRD(ctx context.Context, gvr schema.GroupVersionResource, fns cache.ResourceEventHandlerFuncs) {
	cfg := utils.InClusterConfigOrDie()

	// 创建动态客户端
	dynamicClient := dynamic.NewForConfigOrDie(cfg)
	// 创建动态 informer 工厂
	informerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
	// 创建针对指定 GVR 的 informer
	informer := informerFactory.ForResource(gvr).Informer()
	// 注册事件处理函数
	informer.AddEventHandler(fns)
	// 启动 informer 并等待缓存同步
	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())
}

// WatchEndpointSlice 阻塞监听 EndpointSlice 的变化，并通过事件处理函数 fns 进行处理
func WatchEndpointSlice(ctx context.Context, fns cache.ResourceEventHandlerFuncs) {
	cfg := utils.InClusterConfigOrDie()

	// 创建 Kubernetes 客户端
	client := kubernetes.NewForConfigOrDie(cfg)
	// 创建 SharedInformerFactory
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	// 创建 EndpointSlice 的 informer
	informer := informerFactory.Discovery().V1().EndpointSlices().Informer()
	// 注册事件处理函数
	informer.AddEventHandler(fns)
	// 启动 informer 并等待缓存同步
	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())
}

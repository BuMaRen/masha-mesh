package ctrl

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// WatchCRD 阻塞监听指定 CRD 的变化，并通过事件处理函数 fns 进行处理
func WatchCRD(ctx context.Context, dynamicClient *dynamic.DynamicClient, gvr schema.GroupVersionResource, fns cache.ResourceEventHandlerFuncs) {
	// 创建动态 informer 工厂
	informerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
	// 创建针对指定 GVR 的 informer
	informer := informerFactory.ForResource(gvr).Informer()
	// 注册事件处理函数
	informer.AddEventHandler(fns)
	// 启动 informer 并等待缓存同步
	informerFactory.Start(ctx.Done())
	syncedMap := informerFactory.WaitForCacheSync(ctx.Done())
	if synced, ok := syncedMap[gvr]; !ok || !synced {
		if ctx.Err() != nil {
			klog.V(4).Infof("[Informer] context canceled before cache sync for GVR: %v", gvr)
			return
		}
		klog.Errorf("[Informer] failed to sync cache for GVR: %v", gvr)
		return
	}
	klog.V(4).Infof("[Informer] cache synced for GVR: %v", gvr)
}

// WatchEndpointSlice 阻塞监听 EndpointSlice 的变化，并通过事件处理函数 fns 进行处理
func WatchEndpointSlice(ctx context.Context, k8sClient *kubernetes.Clientset, fns cache.ResourceEventHandlerFuncs) {
	// 创建 SharedInformerFactory
	informerFactory := informers.NewSharedInformerFactory(k8sClient, 0)
	// 创建 EndpointSlice 的 informer
	informer := informerFactory.Discovery().V1().EndpointSlices().Informer()
	// 注册事件处理函数
	informer.AddEventHandler(fns)
	// 启动 informer 并等待缓存同步
	informerFactory.Start(ctx.Done())
	for t, ok := range informerFactory.WaitForCacheSync(ctx.Done()) {
		if !ok {
			if ctx.Err() != nil {
				klog.V(4).Infof("[Informer] context canceled before cache sync for type: %v", t)
				continue
			}
			klog.Errorf("[Informer] failed to sync cache for type: %v", t)
			continue
		}
		klog.V(4).Infof("[Informer] cache synced for type: %v", t)
	}
}

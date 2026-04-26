package ctrl

import (
	"context"
	"fmt"
	"net"

	"github.com/BuMaRen/mesh/pkg/ctrl/utils"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

/*
Logic 负责：
1. 拉起 distributer 作为 grpcServer
2. 拉起 informer 监听 EndpointSlice 的变化，并将变化同步到 storage 中
3. 将 grpcServer 作为 distributer 注册到 storage 中
*/

type Logic struct {
	grpcPort             int
	core                 Storage
	compeletedGrpcServer *grpc.Server
}

// TODO: 待完成
func (l *Logic) WatchCRD(ctx context.Context) {
	kubeConfig := utils.InClusterConfigOrDie()

	dynamicClient := dynamic.NewForConfigOrDie(kubeConfig)

	informerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
	crdInformer := informerFactory.ForResource(schema.GroupVersionResource{
		Group:   "masha.io",
		Version: "v1",
		// 注意这里的 Resource 是复数形式，代表 CRD 的资源类型
		Resource: "containers",
	}).Informer()

	crdInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			// obj -> container
			ctn := parseContainer(obj)
			if ctn != nil {
				klog.Infof("Added container: %s", ctn.Name)
				return
			}
			klog.Warningf("Failed to parse added object as Container: %v", obj)
		},
		UpdateFunc: func(oldObj, newObj any) {
			// oldObj, newObj -> container
			oldCtn := parseContainer(oldObj)
			newCtn := parseContainer(newObj)
			if oldCtn != nil && newCtn != nil {
				klog.Infof("Updated container: %s -> %s", oldCtn.Name, newCtn.Name)
				return
			}
			klog.Warningf("Failed to parse updated objects as Container: %v -> %v", oldObj, newObj)
		},
		DeleteFunc: func(obj any) {
			// obj -> container
			ctn := parseContainer(obj)
			if ctn != nil {
				klog.Infof("Deleted container: %s", ctn.Name)
				return
			}
			klog.Warningf("Failed to parse deleted object as Container: %v", obj)
		},
	})
	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())
}

// WatchEndpointSliceOrDie 启动 informer 监听 EndpointSlice 的变化，并将变化同步到 storage 中
func (l *Logic) WatchEndpointSliceOrDie(ctx context.Context) {
	kubeConfig := utils.InClusterConfigOrDie()

	clientSet := kubernetes.NewForConfigOrDie(kubeConfig)

	informerFactory := informers.NewSharedInformerFactory(clientSet, 0)

	endpointSliceInformer := informerFactory.Discovery().V1().EndpointSlices().Informer()

	endpointSliceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    l.core.OnAdded,
		UpdateFunc: l.core.OnUpdate,
		DeleteFunc: l.core.OnDeleted,
	})

	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())
}

// ServeGrpcOrDie 启动 grpcServer，阻塞监听，直到 context 取消，优雅关闭 grpcServer
func (l *Logic) ServeGrpcOrDie(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", l.grpcPort))
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	go func() {
		<-ctx.Done()
		l.compeletedGrpcServer.GracefulStop()
	}()
	err = l.compeletedGrpcServer.Serve(listener)
	return err
}

func NewLogic(grpcPort int) *Logic {
	distributer := NewGrpcServer()
	storage := NewCoreData(distributer)
	compeletedGrpcServer := distributer.Compelete(storage.List)
	return &Logic{
		grpcPort:             grpcPort,
		core:                 storage,
		compeletedGrpcServer: compeletedGrpcServer,
	}
}

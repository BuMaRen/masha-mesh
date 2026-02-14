package ctrl

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
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

// WatchEndpointSliceOrDie 启动 informer 监听 EndpointSlice 的变化，并将变化同步到 storage 中
func (l *Logic) WatchEndpointSliceOrDie(ctx context.Context) {
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}
	clientSet := kubernetes.NewForConfigOrDie(kubeConfig)

	informerFactory := informers.NewSharedInformerFactory(clientSet, 0)

	endpointSliceInformer := informerFactory.Discovery().V1().EndpointSlices()

	endpointSliceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
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
	compeletedGrpcServer := distributer.Compelete()
	return &Logic{
		grpcPort:             grpcPort,
		core:                 NewCoreData(distributer),
		compeletedGrpcServer: compeletedGrpcServer,
	}
}

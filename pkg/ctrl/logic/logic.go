package logic

import (
	"context"
	"fmt"
	"net"

	"github.com/BuMaRen/mesh/pkg/ctrl"
	"github.com/BuMaRen/mesh/pkg/ctrl/storage"
	"github.com/gin-gonic/gin"
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
	core                 ctrl.Storage
	compeletedGrpcServer *grpc.Server
	httpsServer          *HttpsServer
}

func (l *Logic) Compelete(opts *Options) error {
	// https 服务器配置相关，本包调用不用NewHttpsServer，直接在这里初始化，减少包之间的依赖
	l.httpsServer = &HttpsServer{
		address:  opts.Address,
		certFile: opts.Crt,
		keyFile:  opts.Key,

		aggregator: func(e *gin.Engine) {
			Aggregation(e, opts.InjectionImageTag, opts.InjectionCommand)
		},
	}

	l.grpcPort = opts.GrpcPort

	// grpc 服务器配置相关
	distributer := NewGrpcServer(opts.MapInitialSize)
	coreData := storage.NewCoreData(distributer)
	l.core = coreData
	l.compeletedGrpcServer = distributer.Compelete(coreData.List)
	return nil
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

func (l *Logic) ServeHttpsOrDie(ctx context.Context) error {
	return l.httpsServer.Serve(ctx)
}

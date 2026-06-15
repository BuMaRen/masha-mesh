package grpcserver

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/BuMaRen/mesh/internal/ctrl/utils"
	"github.com/BuMaRen/mesh/pkg/api/mesh"
	"github.com/BuMaRen/mesh/pkg/metrics"
	pubutils "github.com/BuMaRen/mesh/pkg/utils"
	"google.golang.org/grpc"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/klog/v2"
)

const mapInitialSize = 100

type SidecarInformer func(any)

// GrpcServer 实现 Distributer 接口，维护一个 sidecar 列表和对应的 channel
type GrpcServer struct {
	listFn   func(string) (*discoveryv1.EndpointSlice, bool)
	sidecars map[string]*utils.Sidecar
	mtx      *sync.RWMutex
	ready    atomic.Bool
	mesh.UnimplementedMeshCtrlServer
}

func (s *GrpcServer) GetSidecar(svcName string) *utils.Sidecar {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	for _, sidecar := range s.sidecars {
		if sidecar.SubServiceName == svcName {
			return sidecar
		}
	}
	return nil
}

// RPC:Subcribe sidecar 订阅某个 service 的 EndpointSlice 变化事件，服务端通过 channel 推送消息
func (s *GrpcServer) Subscribe(sr *mesh.SubscriptionRequest, sss grpc.ServerStreamingServer[mesh.ClientSubscriptionEvent]) error {
	serviceName := sr.ServiceName
	sidecar := utils.NewSidecar(sr.SidecarId, serviceName)

	klog.Infof("[GrpcServer][Subscribe] Sidecar %s subscribed to service %s\n", sr.SidecarId, serviceName)

	// 订阅时先把当前的 EndpointSlice 发给 sidecar，确保 sidecar 能尽快拿到数据
	es, exist := s.listFn(serviceName)
	if exist {
		klog.Infof("[GrpcServer][Subscribe] Sending current EndpointSlice for service %s to sidecar %s\n", serviceName, sr.SidecarId)
		sidecar.Informer(mesh.OpType_ADDED, es)
	}

	s.mtx.Lock()
	s.sidecars[sr.SidecarId] = sidecar
	s.mtx.Unlock()

	metrics.SideCarsConnected.Inc()
	defer metrics.SideCarsConnected.Dec()

	defer func() {
		s.mtx.Lock()
		delete(s.sidecars, sr.SidecarId)
		s.mtx.Unlock()
		klog.Infof("[GrpcServer][Subscribe] Sidecar %s unsubscribed from service %s\n", sr.SidecarId, serviceName)
	}()

	receiver := sidecar.Receiver()
	for event := range receiver {
		if err := sss.Send(event); err != nil {
			klog.Errorf("[GrpcServer][Subscribe] Failed to send event to sidecar %s for service %s: %v\n", sr.SidecarId, serviceName, err)
			return err
		}
		klog.Infof("[GrpcServer][Subscribe] Sent event to sidecar %s for service %s: %+v\n", sr.SidecarId, serviceName, event)
	}
	return nil
}

// IsReady 返回 gRPC 服务器是否已开始监听，供就绪检查使用
func (s *GrpcServer) IsReady() bool {
	return s.ready.Load()
}

func (s *GrpcServer) ListenAndServe(ctx context.Context, opts *Options) error {
	network := opts.network
	address := opts.address

	listener := pubutils.NewListenerOrDie(network, address)
	defer listener.Close()

	grpcServer := grpc.NewServer()
	mesh.RegisterMeshCtrlServer(grpcServer, s)

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	// listener 绑定成功后标记就绪，此时已可接受连接
	s.ready.Store(true)
	defer s.ready.Store(false)
	return grpcServer.Serve(listener)
}

func NewGrpcServer() *GrpcServer {
	return &GrpcServer{
		sidecars: make(map[string]*utils.Sidecar, mapInitialSize),
		mtx:      &sync.RWMutex{},
	}
}

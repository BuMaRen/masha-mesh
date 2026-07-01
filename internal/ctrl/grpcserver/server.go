package grpcserver

import (
	"context"
	"fmt"
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
	mtx      *sync.Mutex
	ready    *atomic.Bool
	mesh.UnimplementedMeshCtrlServer
}

func (s *GrpcServer) GetSidecar(svcName string) *utils.Sidecar {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	for _, sidecar := range s.sidecars {
		if sidecar.SubServiceName == svcName {
			return sidecar
		}
	}
	return nil
}

// RPC:Subcribe sidecar 订阅某个 service 的 EndpointSlice 变化事件，服务端通过 channel 推送消息
func (s *GrpcServer) Subscribe(sr *mesh.SubscriptionRequest, sss grpc.ServerStreamingServer[mesh.ClientSubscriptionEvent]) error {
	if !s.ready.Load() {
		klog.Warningf("[GrpcServer][Subscribe] server not ready, rejecting subscription from sidecar %s for service %s", sr.SidecarId, sr.ServiceName)
		return fmt.Errorf("server not ready, please try again later")
	}
	serviceName := sr.ServiceName
	sidecar := utils.NewSidecar(sr.SidecarId, serviceName)
	klog.Infof("[GrpcServer][Subscribe] sidecar %s subscribed to service %s", sr.SidecarId, serviceName)

	// 订阅时先把当前的 EndpointSlice 发给 sidecar，确保 sidecar 能尽快拿到数据
	if es, exist := s.listFn(serviceName); exist {
		klog.Infof("[GrpcServer][Subscribe] sending current EndpointSlice for service %s to sidecar %s", serviceName, sr.SidecarId)
		sidecar.Informer(mesh.OpType_ADDED, es)
	}

	// 在锁内二次检查 ready 并插入 map，与 shutdown goroutine 的 ready.Store(false)+Close 互斥，
	// 避免 sidecar 在 CloseReceivers 之后才入 map 导致 channel 永远不被关闭
	s.mtx.Lock()
	if !s.ready.Load() {
		s.mtx.Unlock()
		klog.Warningf("[GrpcServer][Subscribe] server not ready, rejecting subscription from sidecar %s for service %s", sr.SidecarId, sr.ServiceName)
		return fmt.Errorf("server not ready, please try again later")
	}
	s.sidecars[sr.SidecarId] = sidecar
	s.mtx.Unlock()

	metrics.SideCarsConnected.Inc()
	defer metrics.SideCarsConnected.Dec()

	defer func() {
		s.mtx.Lock()
		delete(s.sidecars, sr.SidecarId)
		s.mtx.Unlock()
		klog.Infof("[GrpcServer][Subscribe] sidecar %s unsubscribed from service %s", sr.SidecarId, serviceName)
	}()

	receiver := sidecar.Receiver()
	for event := range receiver {
		if err := sss.Send(event); err != nil {
			klog.Errorf("[GrpcServer][Subscribe] failed to send event to sidecar %s for service %s: %v", sr.SidecarId, serviceName, err)
			return err
		}
		klog.V(4).Infof("[GrpcServer][Subscribe] sent event to sidecar %s for service %s: %+v", sr.SidecarId, serviceName, event)
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
		s.mtx.Lock()

		// 这之后不再允许新的 sidecar 订阅，防止在关闭过程中出现新的订阅
		s.ready.Store(false)
		receivers := []*utils.Sidecar{}
		for _, sidecar := range s.sidecars {
			receivers = append(receivers, sidecar)
		}
		for _, receiver := range receivers {
			receiver.Close()
		}

		s.mtx.Unlock()
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
		ready:    &atomic.Bool{},
		mtx:      &sync.Mutex{},
	}
}

package ctrl

import (
	"sync"

	"github.com/BuMaRen/mesh/pkg/api/mesh"
	"github.com/BuMaRen/mesh/pkg/ctrl/utils"
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
	mesh.UnimplementedMeshCtrlServer
}

// Publish 给所有的 sidecar 注册进来的 channel 分发消息，理应不阻塞
func (s *GrpcServer) Publish(svcName string, opType mesh.OpType, obj any) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	klog.Infof("[GrpcServer][Publish] Publishing update for service %s to %d sidecars\n", svcName, len(s.sidecars))
	for sidecarID, sidecar := range s.sidecars {
		klog.Infof("[GrpcServer][Publish] Publishing update for service %s to sidecar %s(sub service: %s)\n", svcName, sidecarID, sidecar.SubServiceName)
		if sidecar.SubServiceName == svcName {
			sidecar.Informer(opType, obj)
		}
	}
	klog.Infof("[GrpcServer][Publish] Finished publishing update for service %s to all sidecars\n", svcName)
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

func NewGrpcServer() *GrpcServer {
	return &GrpcServer{
		sidecars: make(map[string]*utils.Sidecar, mapInitialSize),
		mtx:      &sync.RWMutex{},
	}
}

func (s *GrpcServer) Compelete(fn func(string) (*discoveryv1.EndpointSlice, bool)) *grpc.Server {
	s.listFn = fn
	grpcServer := grpc.NewServer()
	mesh.RegisterMeshCtrlServer(grpcServer, s)
	return grpcServer
}

func NewCompletedGrpcServer() *grpc.Server {
	grpcServer := grpc.NewServer()
	mesh.RegisterMeshCtrlServer(grpcServer, NewGrpcServer())
	return grpcServer
}

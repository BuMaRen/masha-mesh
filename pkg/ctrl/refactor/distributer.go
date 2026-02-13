package ctrl

import (
	"sync"

	"github.com/BuMaRen/mesh/pkg/api/mesh"
	"github.com/BuMaRen/mesh/pkg/ctrl/refactor/utils"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

const mapInitialSize = 100

type SidecarInformer func(any)

// GrpcServer 实现 Distributer 接口，维护一个 sidecar 列表和对应的 channel
type GrpcServer struct {
	sidecars map[string]*utils.Sidecar
	mtx      *sync.RWMutex
	mesh.UnimplementedMeshCtrlServer
}

// Publish 给所有的 sidecar 注册进来的 channel 分发消息，理应不阻塞
func (s *GrpcServer) Publish(svcName string, obj any) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	klog.Infof("[GrpcServer][Publish] Publishing update for service %s to %d sidecars\n", svcName, len(s.sidecars))
	for sidecarID, sidecar := range s.sidecars {
		if sidecar.SubServiceName == svcName {
			sidecar.Informer(obj)
			klog.Infof("[GrpcServer][Publish] Published update for service %s to sidecar %s\n", svcName, sidecarID)
		}
	}
	klog.Infof("[GrpcServer][Publish] Finished publishing update for service %s to all sidecars\n", svcName)
}

// RPC:Subcribe sidecar 订阅某个 service 的 EndpointSlice 变化事件，服务端通过 channel 推送消息
func (s *GrpcServer) Subscribe(sr *mesh.SubscriptionRequest, sss grpc.ServerStreamingServer[mesh.ClientSubscriptionEvent]) error {
	sidecar := utils.NewSidecar(sr.SidecarId)
	serviceName := sr.ServiceName

	s.mtx.Lock()
	s.sidecars[sr.SidecarId] = sidecar
	s.mtx.Unlock()

	defer func() {
		s.mtx.Lock()
		delete(s.sidecars, sr.SidecarId)
		s.mtx.Unlock()
	}()

	receiver := sidecar.Receiver()
	for event := range receiver {
		if err := sss.Send(event); err != nil {
			klog.Errorf("[GrpcServer][Subscribe] Failed to send event to sidecar %s for service %s: %v\n", sr.SidecarId, serviceName, err)
			return err
		}
	}
	return nil
}

func NewGrpcServer() *GrpcServer {
	return &GrpcServer{
		sidecars: make(map[string]*utils.Sidecar, mapInitialSize),
		mtx:      &sync.RWMutex{},
	}
}

func NewCompletedGrpcServer() *grpc.Server {
	grpcServer := grpc.NewServer()
	mesh.RegisterMeshCtrlServer(grpcServer, NewGrpcServer())
	return grpcServer
}

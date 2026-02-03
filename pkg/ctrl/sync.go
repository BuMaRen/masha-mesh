package ctrl

import (
	"fmt"
	"sync"

	"github.com/BuMaRen/mesh/pkg/ctrl/mesh"
	"google.golang.org/grpc"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// 对应单个 Service 的 EndpointSlice 变化事件
// sidecar 存储 map[ServiceName]map[EndpointName]EndpointIPs
// 每次推送的事件包含的 Endpoints 用于 sidecar 更新
func newClientSubscriptionEvent(revision int64, opType mesh.OpType, es *discoveryv1.EndpointSlice) *mesh.ClientSubscriptionEvent {
	endpoints := make(map[string]*mesh.EndpointIPs)
	for _, endpoint := range es.Endpoints {
		endpointName := string(endpoint.TargetRef.UID)
		endpoints[endpointName] = &mesh.EndpointIPs{
			EndpointIps: endpoint.Addresses,
		}
	}
	return &mesh.ClientSubscriptionEvent{
		Revision:  revision,
		OpType:    opType,
		Endpoints: endpoints,
	}
}

type SidecarID string
type SidecarChannel chan *mesh.ClientSubscriptionEvent

// 一个 ServiceSubscription 表示一个 Service 的订阅
type ServiceSubscription struct {
	// 确保 remove 在关闭 channel 的时候没有 informer 在写
	publishMtx sync.RWMutex
	// 确保 clients 修改的并发安全
	serviceMtx sync.RWMutex
	clients    map[SidecarID]SidecarChannel
}

func newServiceSubscription() *ServiceSubscription {
	return &ServiceSubscription{
		serviceMtx: sync.RWMutex{},
		clients:    make(map[SidecarID]SidecarChannel),
	}
}

func (s *ServiceSubscription) newSidecarChannel(sidecarID SidecarID) SidecarChannel {
	ch := make(SidecarChannel, 100)
	s.serviceMtx.Lock()
	s.clients[sidecarID] = ch
	s.serviceMtx.Unlock()
	return ch
}

func (s *ServiceSubscription) removeSidecarChannel(sidecarID SidecarID) int {
	s.serviceMtx.Lock()
	defer s.serviceMtx.Unlock()
	if ch, ok := s.clients[sidecarID]; ok {
		close(ch)
	}
	delete(s.clients, sidecarID)
	length := len(s.clients)
	return length
}

// 作为 informer 注册到 data 里
func (s *ServiceSubscription) Publish(revision int64, et watch.EventType, es *discoveryv1.EndpointSlice) {
	var opType mesh.OpType
	switch et {
	case watch.Added:
		opType = mesh.OpType_ADDED
	case watch.Modified:
		opType = mesh.OpType_MODIFIED
	case watch.Deleted:
		opType = mesh.OpType_DELETED
	}
	event := newClientSubscriptionEvent(revision, opType, es)
	fmt.Printf("start publish update event, revision(%v)\n", revision)
	s.serviceMtx.Lock()
	for _, ch := range s.clients {
		ch <- event
	}
	s.serviceMtx.Unlock()
	fmt.Printf("update event published, revision(%v)\n", revision)
}

type Informer interface {
	AddInformer(ServiceName, informer)
	DelInformer(ServiceName)
}

type Sync struct {
	mtx           sync.RWMutex
	informer      Informer
	subscriptions map[ServiceName]*ServiceSubscription
	mesh.UnimplementedMeshCtrlServer
}

func NewSync(informer Informer) *Sync {
	return &Sync{
		mtx:           sync.RWMutex{},
		informer:      informer,
		subscriptions: make(map[ServiceName]*ServiceSubscription),
	}
}

func (s *Sync) ServiceSubscriptionOrNew(serviceName ServiceName) *ServiceSubscription {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	svcSub, exist := s.subscriptions[serviceName]
	if !exist {
		s.subscriptions[serviceName] = newServiceSubscription()
		svcSub = s.subscriptions[serviceName]
		s.informer.AddInformer(serviceName, svcSub.Publish)
	}
	return svcSub
}

func (s *Sync) Subscribe(sr *mesh.SubscriptionRequest, sss grpc.ServerStreamingServer[mesh.ClientSubscriptionEvent]) error {
	svcName := ServiceName(sr.ServiceName)
	svcSub := s.ServiceSubscriptionOrNew(svcName)
	recvCh := svcSub.newSidecarChannel(SidecarID(sr.SidecarId))
	defer func() {
		// 删除 sidecar 的通道后如果服务为空
		if remain := svcSub.removeSidecarChannel(SidecarID(sr.SidecarId)); remain == 0 {
			s.informer.DelInformer(svcName)
			s.mtx.Lock()
			delete(s.subscriptions, svcName)
			s.mtx.Unlock()
		}
	}()
	fmt.Printf("start streaming message to sidecar %v...\n", sr.SidecarId)
	for {
		event, readable := <-recvCh
		if !readable {
			fmt.Println("stream finished, rpc exit")
			break
		}
		fmt.Printf("receive updated event for sidecar %v: %+v\n", sr.SidecarId, event)
		err := sss.Send(event)
		if err != nil {
			fmt.Printf("send message to sidecar failed: %v\n", err)
			return err
		}
	}
	return nil
}

package ctrl

import (
	"fmt"
	"sync"

	"github.com/BuMaRen/mesh/pkg/api/mesh"
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

const cacheCapacity = 100

// 一个 ServiceSubscription 表示一个 Service 的订阅
type ServiceSubscription struct {
	channelCapacity int
	// 确保 clients 修改的并发安全
	serviceMtx sync.RWMutex
	clients    map[SidecarID]SidecarChannel
}

func newServiceSubscription() *ServiceSubscription {
	return &ServiceSubscription{
		channelCapacity: cacheCapacity,
		serviceMtx:      sync.RWMutex{},
		clients:         make(map[SidecarID]SidecarChannel),
	}
}

// 新增 SidecarChannel 的时候上锁，不允许推送和删除
func (s *ServiceSubscription) newSidecarChannel(sidecarID SidecarID) SidecarChannel {
	ch := make(SidecarChannel, s.channelCapacity)
	s.serviceMtx.Lock()
	fmt.Printf("[ServiceSubscription][newSidecarChannel] Lock acquired for sidecar %v addition\n", sidecarID)
	defer func() {
		s.serviceMtx.Unlock()
		fmt.Printf("[ServiceSubscription][newSidecarChannel] Lock released after sidecar %v addition\n", sidecarID)
	}()
	s.clients[sidecarID] = ch
	return ch
}

// 删除 SidecarChannel 的时候上锁，不允许推送和新增
func (s *ServiceSubscription) removeSidecarChannel(sidecarID SidecarID) int {
	s.serviceMtx.Lock()
	fmt.Printf("[ServiceSubscription][removeSidecarChannel] Lock acquired for sidecar %v removal\n", sidecarID)
	defer func() {
		s.serviceMtx.Unlock()
		fmt.Printf("[ServiceSubscription][removeSidecarChannel] Lock released after sidecar %v removal\n", sidecarID)
	}()
	if ch, ok := s.clients[sidecarID]; ok {
		close(ch)
	}
	delete(s.clients, sidecarID)
	length := len(s.clients)
	return length
}

// data 在调用 publish 时候会起一个新的 goroutine
// publish 在推送事件的时候不允许修改 clients 列表
func (s *ServiceSubscription) publish(revision int64, et watch.EventType, es *discoveryv1.EndpointSlice) {
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
	// 不用拷贝 clients，防止另一个 goroutine 调用 removeSidecarChannel 关闭 channel 导致 panic
	s.serviceMtx.Lock()
	fmt.Printf("[ServiceSubscription][publish] Lock acquired for publishing revision %v\n", revision)
	for _, ch := range s.clients {
		ch <- event
	}
	s.serviceMtx.Unlock()
	fmt.Printf("[ServiceSubscription][publish] Lock released after publishing revision %v\n", revision)
}

type Storage interface {
	DelBroadcast(ServiceName)
	AddBroadcast(ServiceName, broadcaster)
	Initialize(ServiceName) *discoveryv1.EndpointSlice
}

type Sync struct {
	mtx           sync.RWMutex
	storage       Storage
	subscriptions map[ServiceName]*ServiceSubscription
	mesh.UnimplementedMeshCtrlServer
}

func NewSync(storage Storage) *Sync {
	return &Sync{
		mtx:           sync.RWMutex{},
		storage:       storage,
		subscriptions: make(map[ServiceName]*ServiceSubscription),
	}
}

// 需要订阅新的 Service，整个 Sync 加锁，阻塞各个 goroutine 中的 delete
func (s *Sync) ServiceSubscriptionOrNew(serviceName ServiceName) *ServiceSubscription {
	s.mtx.Lock()
	fmt.Printf("[Sync][ServiceSubscriptionOrNew] Lock acquired for service %v\n", serviceName)
	defer func() {
		s.mtx.Unlock()
		fmt.Printf("[Sync][ServiceSubscriptionOrNew] Lock released for service %v\n", serviceName)
	}()
	svcSub, exist := s.subscriptions[serviceName]
	if !exist {
		s.subscriptions[serviceName] = newServiceSubscription()
		svcSub = s.subscriptions[serviceName]
		// 涉及 storage 中的锁
		s.storage.AddBroadcast(serviceName, svcSub.publish)
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
			s.storage.DelBroadcast(svcName)
			s.mtx.Lock()
			fmt.Printf("[Sync][Subscribe] Lock acquired for service %v deletion\n", svcName)
			delete(s.subscriptions, svcName)
			s.mtx.Unlock()
			fmt.Printf("[Sync][Subscribe] Lock released for service %v deletion\n", svcName)
		}
	}()
	fmt.Printf("start streaming message to sidecar %v...\n", sr.SidecarId)
	endpointSlice := s.storage.Initialize(svcName)
	initEvent := newClientSubscriptionEvent(0, mesh.OpType_ADDED, endpointSlice)
	if err := sss.Send(initEvent); err != nil {
		fmt.Printf("send message to sidecar failed: %v\n", err)
		return err
	}
	for done := false; !done; {
		select {
		case <-sss.Context().Done():
			fmt.Printf("sidecar %v disconnected, stop streaming\n", sr.SidecarId)
			done = true
		case event := <-recvCh:
			fmt.Printf("receive updated event for sidecar %v: %+v\n", sr.SidecarId, event)
			if err := sss.Send(event); err != nil {
				fmt.Printf("send message to sidecar failed: %v\n", err)
				return err
			}
		}
	}
	return nil
}

func (s *Sync) NewGrpcServer() *grpc.Server {
	gSvr := grpc.NewServer()
	mesh.RegisterMeshCtrlServer(gSvr, s)
	return gSvr
}

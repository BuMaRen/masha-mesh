package ctrl

import (
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type EndpointName string
type EndpointIPs []string
type ClientSubscriptionEvent struct {
	Revision  int64
	OpType    string
	Endpoints map[EndpointName]EndpointIPs
}

// 对应单个 Service 的 EndpointSlice 变化事件
// sidecar 存储 map[ServiceName]map[EndpointName]EndpointIPs
// 每次推送的事件包含的 Endpoints 用于 sidecar 更新
func newClientSubscriptionEvent(revision int64, opType string, es *discoveryv1.EndpointSlice) *ClientSubscriptionEvent {
	endpoints := make(map[EndpointName]EndpointIPs)
	for _, endpoint := range es.Endpoints {
		endpointName := EndpointName(endpoint.TargetRef.UID)
		endpoints[endpointName] = endpoint.Addresses
	}
	return &ClientSubscriptionEvent{
		Revision:  revision,
		OpType:    opType,
		Endpoints: endpoints,
	}
}

type SidecarID string
type SidecarChannel chan *ClientSubscriptionEvent

type ServiceSubscription struct {
	clients map[SidecarID]SidecarChannel
}

func (s *ServiceSubscription) Informer(i int64, et watch.EventType, es *discoveryv1.EndpointSlice) {
	event := newClientSubscriptionEvent(i, string(et), es)
	// 遍历所有 goroutine 添加的 channel，发送事件
	for _, ch := range s.clients {
		ch <- event
	}
}

type Sync struct {
}

func (s *Sync) newInformer(serviceName ServiceName) informer {
	return func(i int64, et watch.EventType, es *discoveryv1.EndpointSlice) {

	}
}

func (s *Sync) Informer(revision int64, eventType watch.EventType, eps *discoveryv1.EndpointSlice) {

}

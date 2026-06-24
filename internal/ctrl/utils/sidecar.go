package utils

import (
	"sync"

	"github.com/BuMaRen/mesh/pkg/api/mesh"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/klog/v2"
)

type Sidecar struct {
	closed         bool
	Name           string
	SubServiceName string
	informerMtx    *sync.Mutex
	receiver       chan *mesh.ClientSubscriptionEvent
}

const channelCapacity = 100

func NewSidecar(name, subService string) *Sidecar {
	return &Sidecar{
		Name:           name,
		SubServiceName: subService,
		informerMtx:    &sync.Mutex{},
		receiver:       make(chan *mesh.ClientSubscriptionEvent, channelCapacity),
	}
}

// Informer 把 obj 写到 channel 中，阻塞则跳过
func (s *Sidecar) Informer(opType mesh.OpType, obj any) {
	if obj == nil {
		klog.Warningf("[Sidecar][Informer] received nil object for service %s in sidecar %s, skipping", s.SubServiceName, s.Name)
		return
	}
	endpointSlice, ok := obj.(*discoveryv1.EndpointSlice)
	if !ok {
		klog.Warningf("[Sidecar][Informer] failed to cast object to EndpointSlice for service %s in sidecar %s, skipping", s.SubServiceName, s.Name)
		return
	}
	svcName := endpointSlice.Labels["kubernetes.io/service-name"]
	if svcName != s.SubServiceName {
		klog.V(4).Infof("[Sidecar][Informer] received update for service %s which does not match subscribed service %s in sidecar %s, skipping", svcName, s.SubServiceName, s.Name)
		return
	}
	protoMsg := newClientSubscriptionEvent(opType, endpointSlice)
	s.informerMtx.Lock()
	defer s.informerMtx.Unlock()
	if s.closed {
		klog.Warningf("[Sidecar][Informer] sidecar %s is closed, skipping update for service %s", s.Name, svcName)
		return
	}
	select {
	case s.receiver <- protoMsg:
		klog.Infof("[Sidecar][Informer] sent update for service %s to sidecar %s", svcName, s.Name)
	default:
		klog.Warningf("[Sidecar][Informer] skipping update for service %s to sidecar %s: channel full", svcName, s.Name)
	}
}

// Receiver 返回一个只读的 channel，供 sidecar 监听
func (s *Sidecar) Receiver() <-chan *mesh.ClientSubscriptionEvent {
	return s.receiver
}

func (s *Sidecar) Close() {
	s.informerMtx.Lock()
	defer s.informerMtx.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.receiver)
}

func newClientSubscriptionEvent(opType mesh.OpType, es *discoveryv1.EndpointSlice) *mesh.ClientSubscriptionEvent {
	endpoints := make(map[string]*mesh.EndpointIPs)
	for _, endpoint := range es.Endpoints {
		endpointName := string(endpoint.TargetRef.UID)
		endpoints[endpointName] = &mesh.EndpointIPs{
			EndpointIps: endpoint.Addresses,
		}
	}
	return &mesh.ClientSubscriptionEvent{
		OpType:    opType,
		Endpoints: endpoints,
	}
}

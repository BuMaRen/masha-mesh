package utils

import (
	"github.com/BuMaRen/mesh/pkg/api/mesh"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/klog/v2"
)

type Sidecar struct {
	Name           string
	SubServiceName string
	receiver       chan *mesh.ClientSubscriptionEvent
}

const channelCapacity = 100

func NewSidecar(name string) *Sidecar {
	return &Sidecar{
		Name:     name,
		receiver: make(chan *mesh.ClientSubscriptionEvent, channelCapacity),
	}
}

// Informer 把 obj 写到 channel 中，阻塞则跳过
func (s *Sidecar) Informer(obj any) {
	if obj == nil {
		// Handle nil object case
		return
	}
	endpointSlice, ok := obj.(*discoveryv1.EndpointSlice)
	if !ok {
		// Handle type assertion failure
		return
	}
	svcName := endpointSlice.Labels["kubernetes.io/service-name"]
	if svcName != s.SubServiceName {
		// Not interested in this service
		return
	}
	protoMsg := newClientSubscriptionEvent(endpointSlice)
	select {
	case s.receiver <- protoMsg:
		klog.Infof("[Sidecar][Informer] Sent update for service %s to sidecar %s\n", svcName, s.Name)
	default:
		klog.Warningf("[Sidecar][Informer] Skipping update for service %s to sidecar %s due to full channel\n", svcName, s.Name)
	}
}

// Receiver 返回一个只读的 channel，供 sidecar 监听
func (s *Sidecar) Receiver() <-chan *mesh.ClientSubscriptionEvent {
	return s.receiver
}

func newClientSubscriptionEvent(es *discoveryv1.EndpointSlice) *mesh.ClientSubscriptionEvent {
	endpoints := make(map[string]*mesh.EndpointIPs)
	for _, endpoint := range es.Endpoints {
		endpointName := string(endpoint.TargetRef.UID)
		endpoints[endpointName] = &mesh.EndpointIPs{
			EndpointIps: endpoint.Addresses,
		}
	}
	return &mesh.ClientSubscriptionEvent{
		Endpoints: endpoints,
	}
}

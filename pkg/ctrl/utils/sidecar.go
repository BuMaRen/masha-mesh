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

func NewSidecar(name, subService string) *Sidecar {
	return &Sidecar{
		Name:           name,
		SubServiceName: subService,
		receiver:       make(chan *mesh.ClientSubscriptionEvent, channelCapacity),
	}
}

// Informer 把 obj 写到 channel 中，阻塞则跳过
func (s *Sidecar) Informer(obj any) {
	klog.Errorf("[Sidecar][Informer] Start Informer")
	if obj == nil {
		// Handle nil object case
		klog.Warningf("[Sidecar][Informer] Received nil object for service %s in sidecar %s, skipping\n", s.SubServiceName, s.Name)
		return
	}
	endpointSlice, ok := obj.(*discoveryv1.EndpointSlice)
	if !ok {
		// Handle type assertion failure
		klog.Errorf("[Sidecar][Informer] Failed to cast object to EndpointSlice for service %s in sidecar %s, skipping\n", s.SubServiceName, s.Name)
		return
	}
	svcName := endpointSlice.Labels["kubernetes.io/service-name"]
	if svcName != s.SubServiceName {
		// Not interested in this service
		klog.Infof("[Sidecar][Informer] Received update for service %s which does not match subscribed service %s in sidecar %s, skipping\n", svcName, s.SubServiceName, s.Name)
		return
	}
	protoMsg := newClientSubscriptionEvent(endpointSlice)
	select {
	case s.receiver <- protoMsg:
		klog.Infof("[Sidecar][Informer] Sent update for service %s to sidecar %s\n", svcName, s.Name)
	default:
		klog.Warningf("[Sidecar][Informer] Skipping update for service %s to sidecar %s due to full channel\n", svcName, s.Name)
	}
	klog.Errorf("[Sidecar][Informer] Finished Informer")
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

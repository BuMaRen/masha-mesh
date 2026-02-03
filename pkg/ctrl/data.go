package ctrl

import (
	"fmt"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type ServiceName string

type informer func(int64, watch.EventType, *discoveryv1.EndpointSlice)

type EndpointSliceMap struct {
	revision  int64
	esm       map[ServiceName]*discoveryv1.EndpointSlice
	informers map[ServiceName]informer
}

func (e *EndpointSliceMap) DelInformer(serviceName ServiceName) {
	delete(e.informers, serviceName)
}

func (e *EndpointSliceMap) AddInformer(serviceName ServiceName, fn informer) {
	e.informers[serviceName] = fn
}

func (e *EndpointSliceMap) OnUpdate(event *watch.Event) {
	endpointSlice, ok := event.Object.(*discoveryv1.EndpointSlice)
	if !ok {
		fmt.Printf("event is not endpointSlice")
		return
	}

	serviceName := ServiceName(endpointSlice.Labels["kubernetes.io/service-name"])
	switch event.Type {
	case watch.Added, watch.Modified:
		e.esm[serviceName] = endpointSlice
	case watch.Deleted:
		delete(e.esm, serviceName)
	default:
		fmt.Printf("[EndpointSliceMap] unknown event type: %v\n", event.Type)
		return
	}
	e.revision++
	if fn, exists := e.informers[serviceName]; exists {
		go fn(e.revision, event.Type, endpointSlice.DeepCopy())
	}
}

// mashazheng@Mashas-Air ~ % kubectl get endpointslice kubernetes -o yaml
// addressType: IPv4
// apiVersion: discovery.k8s.io/v1
// endpoints:
// - addresses:
//   - 192.168.49.2
//   conditions:
//     ready: true
// kind: EndpointSlice
// metadata:
//   creationTimestamp: "2026-02-02T17:24:23Z"
//   generation: 1
//   labels:
//     kubernetes.io/service-name: kubernetes
//   name: kubernetes
//   namespace: default
//   resourceVersion: "205"
//   uid: b0328611-f51f-45c9-951b-9a857cd73def
// ports:
// - name: https
//   port: 8443
//   protocol: TCP

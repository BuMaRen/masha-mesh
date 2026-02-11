package ctrl

import (
	"fmt"
	"sync"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type ServiceName string

type broadcaster func(int64, watch.EventType, *discoveryv1.EndpointSlice)

type EndpointSliceMap struct {
	revision     int64
	mtx          *sync.RWMutex
	esm          map[ServiceName]*discoveryv1.EndpointSlice
	broadcasters map[ServiceName]broadcaster
}

func (e *EndpointSliceMap) DelBroadcast(serviceName ServiceName) {
	e.mtx.Lock()
	fmt.Printf("[EndpointSliceMap][DelBroadcast] Lock acquired for service %v deletion\n", serviceName)
	delete(e.broadcasters, serviceName)
	e.mtx.Unlock()
	fmt.Printf("[EndpointSliceMap][DelBroadcast] Lock released after service %v deletion\n", serviceName)
}

func (e *EndpointSliceMap) AddBroadcast(serviceName ServiceName, fn broadcaster) {
	e.mtx.Lock()
	fmt.Printf("[EndpointSliceMap][AddBroadcast] Lock acquired for service %v addition\n", serviceName)
	e.broadcasters[serviceName] = fn
	e.mtx.Unlock()
	fmt.Printf("[EndpointSliceMap][AddBroadcast] Lock released after service %v addition\n", serviceName)
}

func (e *EndpointSliceMap) Initialize(serviceName ServiceName) *discoveryv1.EndpointSlice {
	e.mtx.Lock()
	fmt.Printf("[EndpointSliceMap][Initialize] Lock acquired for service %v initialization\n", serviceName)
	defer func() {
		e.mtx.Unlock()
		fmt.Printf("[EndpointSliceMap][Initialize] Lock released after service %v initialization\n", serviceName)
	}()
	out := new(discoveryv1.EndpointSlice)
	if es, exists := e.esm[serviceName]; exists {
		es.DeepCopyInto(out)
	}
	return out
}

func (e *EndpointSliceMap) OnUpdate(event *watch.Event) {
	endpointSlice, ok := event.Object.(*discoveryv1.EndpointSlice)
	if !ok {
		fmt.Printf("event is not endpointSlice")
		return
	}

	serviceName := ServiceName(endpointSlice.Labels["kubernetes.io/service-name"])
	es := endpointSlice.DeepCopy()
	e.mtx.Lock()
	fmt.Printf("[EndpointSliceMap][OnUpdate] Lock acquired for service %v update\n", serviceName)
	defer func() {
		e.mtx.Unlock()
		fmt.Printf("[EndpointSliceMap][OnUpdate] Lock released after service %v update\n", serviceName)
	}()
	switch event.Type {
	case watch.Added, watch.Modified:
		e.esm[serviceName] = es
	case watch.Deleted:
		delete(e.esm, serviceName)
	default:
		fmt.Printf("[EndpointSliceMap] unknown event type: %v\n", event.Type)
		return
	}
	e.revision++
	if fn, exists := e.broadcasters[serviceName]; exists {
		fmt.Printf("[EndpointSliceMap][OnUpdate] start a braodcast for service %v\n", serviceName)
		go fn(e.revision, event.Type, es)
	}
}

func NewEndpointSliceMap() *EndpointSliceMap {
	return &EndpointSliceMap{
		revision:     0,
		mtx:          &sync.RWMutex{},
		esm:          make(map[ServiceName]*discoveryv1.EndpointSlice),
		broadcasters: make(map[ServiceName]broadcaster),
	}
}

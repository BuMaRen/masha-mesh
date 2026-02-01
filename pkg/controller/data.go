package controller

import (
	"fmt"
	"log"
	"sync"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type ServiceName string

func NewData() *Data {
	return &Data{
		revision:          0,
		mtx:               &sync.RWMutex{},
		customers:         make(map[string]chan *informer),
		endpointSliceList: make(map[ServiceName]*discoveryv1.EndpointSlice),
	}
}

type informer struct {
	revision int64
	event    *watch.Event
}

type Data struct {
	customers map[string]chan *informer
	revision  int64
	mtx       *sync.RWMutex
	// add fields here
	endpointSliceList map[ServiceName]*discoveryv1.EndpointSlice
}

// 预期：kubernetes 的 add 事件新增的是一整个 EndpointSlice
func (d *Data) onAdded(epsl *discoveryv1.EndpointSlice) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	serviceName := ServiceName(epsl.Labels["kubernetes.io/service-name"])
	d.endpointSliceList[serviceName] = epsl
}

// 预期： kubernetes 的 delete 事件删除的是一整个 EndpointSlice
func (d *Data) onDeleted(epsl *discoveryv1.EndpointSlice) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	serviceName := ServiceName(epsl.Labels["kubernetes.io/service-name"])
	delete(d.endpointSliceList, serviceName)
}

// 预期： kubernetes 的 modify 事件修改的是一整个 EndpointSlice
func (d *Data) onModified(epsl *discoveryv1.EndpointSlice) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	serviceName := ServiceName(epsl.Labels["kubernetes.io/service-name"])
	d.endpointSliceList[serviceName] = epsl
}

func (d *Data) Update(event *watch.Event) error {
	log.Printf("[data] Update event type=%s", event.Type)
	epsl, ok := event.Object.(*discoveryv1.EndpointSlice)
	if !ok {
		return fmt.Errorf("object is not EndpointSlice")
	}
	svcName := epsl.Labels["kubernetes.io/service-name"]
	log.Printf("[data] Update service=%s endpoints=%d", svcName, len(epsl.Endpoints))
	switch event.Type {
	case watch.Modified:
		log.Printf("[data] switch type Modified")
		d.onModified(epsl)
		log.Printf("[data] switch type Modified finished")
	case watch.Added:
		log.Printf("[data] switch type Added")
		d.onAdded(epsl)
		log.Printf("[data] switch type Added finished")
	case watch.Deleted:
		log.Printf("[data] switch type Deleted")
		d.onDeleted(epsl)
		log.Printf("[data] switch type Deleted finished")
	default:
		return fmt.Errorf("unsupported event type: %s", event.Type)
	}

	d.mtx.Lock()
	d.revision++
	log.Printf("[data] revision=%d customers=%d", d.revision, len(d.customers))
	copyed := make(map[string]chan *informer, len(d.customers))
	for k, v := range d.customers {
		copyed[k] = v
	}
	d.mtx.Unlock()

	// 递增完成后，另起一个goroutine通知所有的客户
	go func() {
		informer := &informer{
			revision: d.revision,
			event:    event,
		}
		log.Printf("[data] broadcasting event type=%s to %d customers", event.Type, len(copyed))
		for _, v := range copyed {
			v <- informer
		}
	}()
	return nil
}

// Register 注册一个客户(一个grpc连接)，并返回当前的 EndpointSlice 列表快照
func (d *Data) Register(customerId string, serviceName ServiceName, eventCh chan *informer) {
	log.Printf("[data] Register customer=%s", customerId)

	d.mtx.Lock()
	d.customers[customerId] = eventCh
	_, exists := d.endpointSliceList[serviceName]
	d.mtx.Unlock()
	if !exists {
		log.Printf("[data] Register no existing service=%s", serviceName)
		return
	}
	go func() {
		info := &informer{
			revision: d.revision,
			event: &watch.Event{
				Type:   watch.Added,
				Object: d.endpointSliceList[serviceName],
			},
		}
		// 这里如果是无缓冲的 channel 会阻塞导致 Subscribe 卡死
		eventCh <- info
	}()
}

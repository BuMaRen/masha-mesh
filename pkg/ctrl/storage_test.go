package ctrl

import (
	"testing"

	"github.com/BuMaRen/mesh/pkg/api/mesh"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type publishEvent struct {
	svcName string
	opType  mesh.OpType
	payload any
}

type mockDistributer struct {
	events []publishEvent
}

func (m *mockDistributer) Publish(svcName string, opType mesh.OpType, payload any) {
	m.events = append(m.events, publishEvent{
		svcName: svcName,
		opType:  opType,
		payload: payload,
	})
}

func makeEndpointSlice(name, serviceName, resourceVersion, ip string) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			ResourceVersion: resourceVersion,
			Labels: map[string]string{
				"kubernetes.io/service-name": serviceName,
			},
		},
		Endpoints: []discoveryv1.Endpoint{{
			Addresses: []string{ip},
		}},
	}
}

func TestNewCoreData(t *testing.T) {
	mock := &mockDistributer{}
	data := NewCoreData(mock)

	if data == nil {
		t.Fatal("expected non-nil CoreData")
	}
	if data.distributer == nil {
		t.Fatal("expected non-nil distributer")
	}
	if data.serviceMap == nil {
		t.Fatal("expected initialized serviceMap")
	}
}

func TestCoreData_List_NotFound(t *testing.T) {
	data := NewCoreData(&mockDistributer{})
	es, ok := data.List("not-exists")

	if ok {
		t.Fatal("expected ok=false for non-existent service")
	}
	if es != nil {
		t.Fatal("expected nil EndpointSlice for non-existent service")
	}
}

func TestCoreData_OnAdded_CreatesServiceAndPublishesAdded(t *testing.T) {
	mock := &mockDistributer{}
	data := NewCoreData(mock)

	obj := makeEndpointSlice("es-1", "svc-a", "1", "10.0.0.1")
	data.OnAdded(obj)

	if _, exist := data.serviceMap["svc-a"]; !exist {
		t.Fatal("expected service svc-a to exist in serviceMap")
	}
	if len(mock.events) != 1 {
		t.Fatalf("expected 1 publish event, got %d", len(mock.events))
	}

	event := mock.events[0]
	if event.svcName != "svc-a" {
		t.Fatalf("expected svcName svc-a, got %s", event.svcName)
	}
	if event.opType != mesh.OpType_ADDED {
		t.Fatalf("expected opType ADDED, got %v", event.opType)
	}

	merged, ok := event.payload.(*discoveryv1.EndpointSlice)
	if !ok {
		t.Fatalf("expected payload type *EndpointSlice, got %T", event.payload)
	}
	if len(merged.Endpoints) != 1 {
		t.Fatalf("expected 1 merged endpoint, got %d", len(merged.Endpoints))
	}
}

func TestCoreData_List_AfterOnAdded(t *testing.T) {
	data := NewCoreData(&mockDistributer{})
	data.OnAdded(makeEndpointSlice("es-1", "svc-a", "1", "10.0.0.1"))
	data.OnAdded(makeEndpointSlice("es-2", "svc-a", "1", "10.0.0.2"))

	es, ok := data.List("svc-a")
	if !ok {
		t.Fatal("expected ok=true for existing service")
	}
	if es == nil {
		t.Fatal("expected non-nil merged EndpointSlice")
	}
	if len(es.Endpoints) != 2 {
		t.Fatalf("expected 2 merged endpoints, got %d", len(es.Endpoints))
	}
}

func TestCoreData_OnUpdate_ExistingService_PublishesModified(t *testing.T) {
	mock := &mockDistributer{}
	data := NewCoreData(mock)

	oldObj := makeEndpointSlice("es-1", "svc-a", "1", "10.0.0.1")
	data.OnAdded(oldObj)
	newObj := makeEndpointSlice("es-1", "svc-a", "2", "10.0.0.9")

	data.OnUpdate(oldObj, newObj)

	if len(mock.events) != 2 {
		t.Fatalf("expected 2 publish events, got %d", len(mock.events))
	}
	last := mock.events[1]
	if last.opType != mesh.OpType_MODIFIED {
		t.Fatalf("expected last opType MODIFIED, got %v", last.opType)
	}
	merged, ok := last.payload.(*discoveryv1.EndpointSlice)
	if !ok {
		t.Fatalf("expected payload type *EndpointSlice, got %T", last.payload)
	}
	if len(merged.Endpoints) != 1 {
		t.Fatalf("expected 1 merged endpoint, got %d", len(merged.Endpoints))
	}
	if len(merged.Endpoints[0].Addresses) != 1 || merged.Endpoints[0].Addresses[0] != "10.0.0.9" {
		t.Fatalf("expected updated address 10.0.0.9, got %+v", merged.Endpoints[0].Addresses)
	}
}

func TestCoreData_OnDeleted_ServiceNotExists(t *testing.T) {
	mock := &mockDistributer{}
	data := NewCoreData(mock)

	data.OnDeleted(makeEndpointSlice("es-1", "svc-not-exists", "1", "10.0.0.1"))

	if len(mock.events) != 0 {
		t.Fatalf("expected no publish event, got %d", len(mock.events))
	}
}

func TestCoreData_OnDeleted_LastSlice_PublishesDeletedAndRemovesService(t *testing.T) {
	mock := &mockDistributer{}
	data := NewCoreData(mock)

	obj := makeEndpointSlice("es-1", "svc-a", "1", "10.0.0.1")
	data.OnAdded(obj)
	data.OnDeleted(obj)

	if len(mock.events) != 2 {
		t.Fatalf("expected 2 publish events, got %d", len(mock.events))
	}
	last := mock.events[1]
	if last.opType != mesh.OpType_DELETED {
		t.Fatalf("expected last opType DELETED, got %v", last.opType)
	}
	if _, exist := data.serviceMap["svc-a"]; exist {
		t.Fatal("expected service svc-a removed from serviceMap")
	}
}

func TestCoreData_OnDeleted_PartialDelete_PublishesModified(t *testing.T) {
	mock := &mockDistributer{}
	data := NewCoreData(mock)

	es1 := makeEndpointSlice("es-1", "svc-a", "1", "10.0.0.1")
	es2 := makeEndpointSlice("es-2", "svc-a", "1", "10.0.0.2")
	data.OnAdded(es1)
	data.OnAdded(es2)

	data.OnDeleted(es1)

	if len(mock.events) != 3 {
		t.Fatalf("expected 3 publish events, got %d", len(mock.events))
	}
	last := mock.events[2]
	if last.opType != mesh.OpType_MODIFIED {
		t.Fatalf("expected last opType MODIFIED, got %v", last.opType)
	}
	if _, exist := data.serviceMap["svc-a"]; !exist {
		t.Fatal("expected service svc-a still exists")
	}

	es, ok := data.List("svc-a")
	if !ok {
		t.Fatal("expected list success for svc-a")
	}
	if len(es.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint after partial delete, got %d", len(es.Endpoints))
	}
}

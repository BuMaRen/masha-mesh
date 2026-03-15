package cli

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/BuMaRen/mesh/pkg/api/mesh"
)

func buildEndpoints(data map[string][]string) *Endpoints {
	event := &mesh.ClientSubscriptionEvent{Endpoints: make(map[string]*mesh.EndpointIPs, len(data))}
	for name, ips := range data {
		event.Endpoints[name] = &mesh.EndpointIPs{EndpointIps: ips}
	}
	return NewEndpoints(event)
}

func TestNewEndpoints(t *testing.T) {
	event := &mesh.ClientSubscriptionEvent{
		Revision: 7,
		OpType:   mesh.OpType_MODIFIED,
		Endpoints: map[string]*mesh.EndpointIPs{
			"eps-a": {EndpointIps: []string{"10.0.0.1"}},
		},
	}

	eps := NewEndpoints(event)
	if eps == nil {
		t.Fatal("expected non-nil endpoints")
	}
	if eps.Revision != event.Revision {
		t.Fatalf("expected revision %d, got %d", event.Revision, eps.Revision)
	}
	if eps.OpType != event.OpType {
		t.Fatalf("expected op type %v, got %v", event.OpType, eps.OpType)
	}
}

func TestEndpoints_GetIps(t *testing.T) {
	eps := buildEndpoints(map[string][]string{
		"eps-a": {"10.0.0.1", "10.0.0.2"},
		"eps-b": {"10.0.1.1"},
	})

	got := eps.GetIps()
	want := map[string][]string{
		"eps-a": {"10.0.0.1", "10.0.0.2"},
		"eps-b": {"10.0.1.1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected ips map, want=%v, got=%v", want, got)
	}
}

func TestEndpoints_GetIpsByName(t *testing.T) {
	eps := buildEndpoints(map[string][]string{
		"eps-a": {"10.0.0.1", "10.0.0.2"},
	})

	got := eps.GetIpsByName("eps-a")
	want := []string{"10.0.0.1", "10.0.0.2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected ips, want=%v, got=%v", want, got)
	}
}

func TestEndpoints_GetIpsByName_NotFound(t *testing.T) {
	eps := buildEndpoints(map[string][]string{
		"eps-a": {"10.0.0.1"},
	})

	got := eps.GetIpsByName("eps-not-found")
	if len(got) != 0 {
		t.Fatalf("expected empty slice for not found endpoint, got=%v", got)
	}
}

func TestServiceCache_OnAdd(t *testing.T) {
	cache := NewServiceCache(2)
	eps := buildEndpoints(map[string][]string{
		"eps-a": {"10.0.0.1"},
	})

	cache.onAdd("svc-a", eps)

	stored, ok := cache.cache["svc-a"]
	if !ok {
		t.Fatal("expected svc-a exists in cache")
	}
	if stored != eps {
		t.Fatal("expected stored endpoints equals added endpoints")
	}
}

func TestServiceCache_OnUpdate(t *testing.T) {
	cache := NewServiceCache(2)
	oldEps := buildEndpoints(map[string][]string{
		"eps-a": {"10.0.0.1"},
	})
	newEps := buildEndpoints(map[string][]string{
		"eps-a": {"10.0.0.9"},
	})

	cache.onAdd("svc-a", oldEps)
	cache.onUpdate("svc-a", oldEps, newEps)

	stored, ok := cache.cache["svc-a"]
	if !ok {
		t.Fatal("expected svc-a exists in cache")
	}
	if stored != newEps {
		t.Fatal("expected stored endpoints equals new endpoints after update")
	}
}

func TestServiceCache_OnUpdate_EmptyEps(t *testing.T) {
	cache := NewServiceCache(2)
	oldEps := buildEndpoints(map[string][]string{
		"eps-a": {"10.0.0.1"},
	})

	cache.onAdd("svc-a", oldEps)
	cache.onUpdate("svc-a")

	stored, ok := cache.cache["svc-a"]
	if !ok {
		t.Fatal("expected svc-a exists in cache")
	}
	if stored != oldEps {
		t.Fatal("expected old endpoints remain unchanged when onUpdate has no eps")
	}
}

func TestServiceCache_OnDelete(t *testing.T) {
	cache := NewServiceCache(2)
	cache.onAdd("svc-a", buildEndpoints(map[string][]string{"eps-a": {"10.0.0.1"}}))

	cache.onDelete("svc-a")

	if _, ok := cache.cache["svc-a"]; ok {
		t.Fatal("expected svc-a removed from cache")
	}
}

func TestServiceCache_GetEndpoints(t *testing.T) {
	cache := NewServiceCache(2)
	eps := buildEndpoints(map[string][]string{"eps-a": {"10.0.0.1"}})
	cache.onAdd("svc-a", eps)

	got := cache.GetEndpoints("svc-a")
	if got == nil {
		t.Fatal("expected non-nil endpoints for existing service")
	}
	if got != eps {
		t.Fatal("expected same endpoints pointer from cache")
	}
}

func TestServiceCache_GetEndpoints_NotFound(t *testing.T) {
	cache := NewServiceCache(2)

	got := cache.GetEndpoints("svc-not-found")
	if got != nil {
		t.Fatalf("expected nil for non-existing service, got=%v", got)
	}
}

func TestServiceCache_GetServiceIps(t *testing.T) {
	cache := NewServiceCache(2)
	eps := buildEndpoints(map[string][]string{
		"eps-a": {"10.0.0.1", "10.0.0.2"},
		"eps-b": {"10.0.1.1"},
	})
	cache.onAdd("svc-a", eps)

	got := cache.GetServiceIps("svc-a")
	want := map[string][]string{
		"eps-a": {"10.0.0.1", "10.0.0.2"},
		"eps-b": {"10.0.1.1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected service ips, want=%v, got=%v", want, got)
	}
}

func TestServiceCache_GetServiceIps_NotFound(t *testing.T) {
	cache := NewServiceCache(2)

	got := cache.GetServiceIps("svc-not-found")
	if len(got) != 0 {
		t.Fatalf("expected empty map for non-existing service, got=%v", got)
	}
}

func TestServiceCache_ConcurrentAccess(t *testing.T) {
	cache := NewServiceCache(4)
	serviceName := "svc-concurrent"

	epsA := buildEndpoints(map[string][]string{"eps-a": {"10.0.0.1"}})
	epsB := buildEndpoints(map[string][]string{"eps-a": {"10.0.0.2"}})

	var wg sync.WaitGroup

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				cache.onAdd(serviceName, epsA)
				cache.onUpdate(serviceName, epsA, epsB)
				if j%10 == 0 {
					cache.onDelete(serviceName)
				}
			}
		}()
	}

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				_ = cache.GetEndpoints(serviceName)
				_ = cache.GetServiceIps(serviceName)
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// pass
	case <-time.After(3 * time.Second):
		t.Fatal("concurrent cache operations timed out")
	}
}

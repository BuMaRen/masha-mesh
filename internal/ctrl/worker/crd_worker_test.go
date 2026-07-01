package worker

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/BuMaRen/mesh/pkg/cache"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8sTesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/workqueue"
)

// zeroRateLimiter is a rate limiter with no delay, for testing only.
type zeroRateLimiter struct {
	mu     sync.Mutex
	counts map[string]int
}

func newZeroRateLimiter() workqueue.TypedRateLimiter[string] {
	return &zeroRateLimiter{counts: make(map[string]int)}
}

func (r *zeroRateLimiter) When(item string) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counts[item]++
	return 0
}

func (r *zeroRateLimiter) Forget(item string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.counts, item)
}

func (r *zeroRateLimiter) NumRequeues(item string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.counts[item]
}

// newTestWorker creates a CRDWorker with a zero-delay queue for use in tests.
func newTestWorker(c *cache.GeneralCache[*corev1.Container], kubeClient *fake.Clientset) *CRDWorker {
	return &CRDWorker{
		queue:      workqueue.NewTypedRateLimitingQueue(newZeroRateLimiter()),
		cache:      c,
		kubeClient: kubeClient,
		label:      "mesh-inject",
		maxRetries: -1,
	}
}

func makeUnstructuredContainer(specName, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "mesh.io/v1",
			"kind":       "Container",
			"metadata": map[string]interface{}{
				"name":      specName,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"name":  specName,
				"image": "nginx:latest",
			},
		},
	}
}

func makeDeployment(name, namespace, label string, containers ...corev1.Container) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{label: "true"},
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: containers,
				},
			},
		},
	}
}

// drainQueue returns the next key from the queue, marks it done, and forgets it.
// It fails the test if the queue is shut down or nothing arrives within 1 second.
func drainQueue(t *testing.T, w *CRDWorker) string {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		key, shutdown := w.queue.Get()
		if shutdown {
			close(done)
			return
		}
		done <- key
	}()
	select {
	case key, ok := <-done:
		if !ok {
			t.Fatal("queue was shut down unexpectedly")
		}
		w.queue.Done(key)
		w.queue.Forget(key)
		return key
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for item in queue")
		return ""
	}
}

// --- TestParseEventKey ---

func TestParseEventKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		want    *CRDWorkerEvent
		wantErr bool
	}{
		{
			name: "valid Add key",
			key:  "Add/default/sidecar",
			want: &CRDWorkerEvent{Type: EventTypeAdd, Namespace: "default", ContainerName: "sidecar"},
		},
		{
			name: "valid Update key",
			key:  "Update/kube-system/mesh-proxy",
			want: &CRDWorkerEvent{Type: EventTypeUpdate, Namespace: "kube-system", ContainerName: "mesh-proxy"},
		},
		{
			name: "valid Delete key with empty namespace",
			key:  "Delete//sidecar",
			want: &CRDWorkerEvent{Type: EventTypeDelete, Namespace: "", ContainerName: "sidecar"},
		},
		{
			name:    "missing parts",
			key:     "Add/default",
			wantErr: true,
		},
		{
			name:    "empty key",
			key:     "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEventKey(tc.key)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for key %q, got nil", tc.key)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// --- TestCRDHandlers_OnAdded ---

func TestCRDHandlers_OnAdded(t *testing.T) {
	c := cache.NewGeneralCache[*corev1.Container]()
	w := newTestWorker(c, fake.NewSimpleClientset())
	h := w.Handlers()

	h.OnAdded(makeUnstructuredContainer("sidecar", "default"))

	// Cache should contain the container.
	got, ok := c.Get("sidecar")
	if !ok {
		t.Fatal("expected container in cache after OnAdded")
	}
	if got.Name != "sidecar" {
		t.Errorf("cache: got name %q, want %q", got.Name, "sidecar")
	}

	// Queue should have the Add key.
	key := drainQueue(t, w)
	if want := "Add/default/sidecar"; key != want {
		t.Errorf("queue key: got %q, want %q", key, want)
	}
}

func TestCRDHandlers_OnAdded_InvalidObj(t *testing.T) {
	c := cache.NewGeneralCache[*corev1.Container]()
	w := newTestWorker(c, fake.NewSimpleClientset())
	h := w.Handlers()

	// Non-unstructured object should be silently ignored.
	h.OnAdded("not-an-unstructured-object")

	if w.queue.Len() != 0 {
		t.Errorf("expected empty queue for invalid object, got len=%d", w.queue.Len())
	}
}

// --- TestCRDHandlers_OnUpdated ---

func TestCRDHandlers_OnUpdated_NotInCache(t *testing.T) {
	c := cache.NewGeneralCache[*corev1.Container]()
	w := newTestWorker(c, fake.NewSimpleClientset())
	h := w.Handlers()

	obj := makeUnstructuredContainer("sidecar", "default")
	// Update without a prior Add → cache.Update returns false → no enqueue.
	h.OnUpdated(obj, obj)

	if w.queue.Len() != 0 {
		t.Errorf("expected empty queue when container not in cache, got len=%d", w.queue.Len())
	}
}

func TestCRDHandlers_OnUpdated(t *testing.T) {
	c := cache.NewGeneralCache[*corev1.Container]()
	w := newTestWorker(c, fake.NewSimpleClientset())
	h := w.Handlers()

	obj := makeUnstructuredContainer("sidecar", "default")
	h.OnAdded(obj)
	drainQueue(t, w) // consume Add event

	h.OnUpdated(nil, obj)

	key := drainQueue(t, w)
	if want := "Update/default/sidecar"; key != want {
		t.Errorf("queue key: got %q, want %q", key, want)
	}
}

// --- TestCRDHandlers_OnDeleted ---

func TestCRDHandlers_OnDeleted_NotInCache(t *testing.T) {
	c := cache.NewGeneralCache[*corev1.Container]()
	w := newTestWorker(c, fake.NewSimpleClientset())
	h := w.Handlers()

	h.OnDeleted(makeUnstructuredContainer("sidecar", "default"))

	if w.queue.Len() != 0 {
		t.Errorf("expected empty queue when container not in cache, got len=%d", w.queue.Len())
	}
}

func TestCRDHandlers_OnDeleted(t *testing.T) {
	c := cache.NewGeneralCache[*corev1.Container]()
	w := newTestWorker(c, fake.NewSimpleClientset())
	h := w.Handlers()

	obj := makeUnstructuredContainer("sidecar", "default")
	h.OnAdded(obj)
	drainQueue(t, w) // consume Add event

	h.OnDeleted(obj)

	// Cache should no longer contain the container.
	if _, ok := c.Get("sidecar"); ok {
		t.Error("expected container to be removed from cache after OnDeleted")
	}

	key := drainQueue(t, w)
	if want := "Delete/default/sidecar"; key != want {
		t.Errorf("queue key: got %q, want %q", key, want)
	}
}

// --- TestCRDWorker_ProcessEvent ---

func TestCRDWorker_ProcessEvent_Add(t *testing.T) {
	c := cache.NewGeneralCache[*corev1.Container]()
	w := newTestWorker(c, fake.NewSimpleClientset())

	// Add event is a no-op (webhook handles injection).
	err := w.processEvent(&CRDWorkerEvent{Type: EventTypeAdd, ContainerName: "sidecar", Namespace: "default"})
	if err != nil {
		t.Errorf("processEvent(Add) returned unexpected error: %v", err)
	}
}

func TestCRDWorker_ProcessEvent_Update(t *testing.T) {
	const (
		namespace     = "default"
		containerName = "sidecar"
		newImage      = "nginx:v2"
		label         = "mesh-inject"
	)

	dep := makeDeployment("my-dep", namespace, label,
		corev1.Container{Name: containerName, Image: "nginx:v1"},
	)
	fakeClient := fake.NewSimpleClientset(dep)

	c := cache.NewGeneralCache[*corev1.Container]()
	newContainer := &corev1.Container{Name: containerName, Image: newImage}
	c.Add(containerName, newContainer)

	w := newTestWorker(c, fakeClient)

	err := w.processEvent(&CRDWorkerEvent{Type: EventTypeUpdate, ContainerName: containerName, Namespace: namespace})
	if err != nil {
		t.Fatalf("processEvent(Update) returned error: %v", err)
	}

	ctx := context.Background()
	updated, err := fakeClient.AppsV1().Deployments(namespace).Get(ctx, "my-dep", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get updated deployment: %v", err)
	}

	for _, c := range updated.Spec.Template.Spec.Containers {
		if c.Name == containerName && c.Image != newImage {
			t.Errorf("container %q image: got %q, want %q", containerName, c.Image, newImage)
		}
	}
}

func TestCRDWorker_ProcessEvent_Update_ContainerNotInCache(t *testing.T) {
	// If the container was deleted from cache before the Update event is processed, skip gracefully.
	c := cache.NewGeneralCache[*corev1.Container]()
	w := newTestWorker(c, fake.NewSimpleClientset())

	err := w.processEvent(&CRDWorkerEvent{Type: EventTypeUpdate, ContainerName: "missing", Namespace: "default"})
	if err != nil {
		t.Errorf("processEvent(Update) with missing cache entry should not error, got: %v", err)
	}
}

func TestCRDWorker_ProcessEvent_Delete(t *testing.T) {
	const (
		namespace     = "default"
		containerName = "sidecar"
		label         = "mesh-inject"
	)

	dep := makeDeployment("my-dep", namespace, label,
		corev1.Container{Name: "app", Image: "app:v1"},
		corev1.Container{Name: containerName, Image: "nginx:v1"},
	)
	fakeClient := fake.NewSimpleClientset(dep)
	c := cache.NewGeneralCache[*corev1.Container]()
	w := newTestWorker(c, fakeClient)

	err := w.processEvent(&CRDWorkerEvent{Type: EventTypeDelete, ContainerName: containerName, Namespace: namespace})
	if err != nil {
		t.Fatalf("processEvent(Delete) returned error: %v", err)
	}

	ctx := context.Background()
	updated, err := fakeClient.AppsV1().Deployments(namespace).Get(ctx, "my-dep", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get updated deployment: %v", err)
	}

	for _, c := range updated.Spec.Template.Spec.Containers {
		if c.Name == containerName {
			t.Errorf("container %q should have been removed from deployment, but still present", containerName)
		}
	}
	if len(updated.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("expected 1 container remaining, got %d", len(updated.Spec.Template.Spec.Containers))
	}
}

// --- TestCRDWorker_Process_Retry ---

func TestCRDWorker_Process_InvalidKey_Dropped(t *testing.T) {
	c := cache.NewGeneralCache[*corev1.Container]()
	w := newTestWorker(c, fake.NewSimpleClientset())

	// Manually add a bad key and process it.
	w.queue.Add("no-slashes-at-all")
	key, _ := w.queue.Get()
	w.process(key)

	// After process, the key should be forgotten (NumRequeues == 0).
	if n := w.queue.NumRequeues(key); n != 0 {
		t.Errorf("invalid key should be dropped immediately, but NumRequeues=%d", n)
	}
}

func TestCRDWorker_Process_RetriesOnError(t *testing.T) {
	const (
		namespace     = "default"
		containerName = "sidecar"
		label         = "mesh-inject"
	)

	dep := makeDeployment("my-dep", namespace, label,
		corev1.Container{Name: containerName, Image: "nginx:v1"},
	)
	fakeClient := fake.NewSimpleClientset(dep)

	// Make all Deployment Update calls fail.
	fakeClient.Fake.PrependReactor("update", "deployments",
		func(_ k8sTesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("simulated API error")
		},
	)

	c := cache.NewGeneralCache[*corev1.Container]()
	c.Add(containerName, &corev1.Container{Name: containerName, Image: "nginx:v2"})
	w := newTestWorker(c, fakeClient)

	key := (&CRDWorkerEvent{Type: EventTypeUpdate, ContainerName: containerName, Namespace: namespace}).key()
	w.queue.Add(key)

	// First attempt: should enqueue for retry.
	rawKey, _ := w.queue.Get()
	w.process(rawKey)

	if n := w.queue.NumRequeues(rawKey); n != 1 {
		t.Errorf("expected 1 requeue after first failure, got %d", n)
	}
}

func TestCRDWorker_Process_ContinuesRetryingOnPersistentError(t *testing.T) {
	const (
		namespace     = "default"
		containerName = "sidecar"
		label         = "mesh-inject"
	)

	dep := makeDeployment("my-dep", namespace, label,
		corev1.Container{Name: containerName, Image: "nginx:v1"},
	)
	fakeClient := fake.NewSimpleClientset(dep)
	fakeClient.Fake.PrependReactor("update", "deployments",
		func(_ k8sTesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("persistent error")
		},
	)

	c := cache.NewGeneralCache[*corev1.Container]()
	c.Add(containerName, &corev1.Container{Name: containerName, Image: "nginx:v2"})
	w := newTestWorker(c, fakeClient)

	key := (&CRDWorkerEvent{Type: EventTypeUpdate, ContainerName: containerName, Namespace: namespace}).key()
	w.queue.Add(key)
	raw1, _ := w.queue.Get()
	w.process(raw1)

	if n := w.queue.NumRequeues(raw1); n != 1 {
		t.Fatalf("expected NumRequeues=1 after first failure, got %d", n)
	}

	// Process the requeued item again, it should keep retrying (not dropped/forgotten).
	raw2, _ := w.queue.Get()
	w.process(raw2)

	if n := w.queue.NumRequeues(raw2); n != 2 {
		t.Errorf("expected NumRequeues=2 after second failure, got %d", n)
	}
}

func TestCRDWorker_Process_DropsAfterConfiguredMaxRetries(t *testing.T) {
	const (
		namespace     = "default"
		containerName = "sidecar"
		label         = "mesh-inject"
	)

	dep := makeDeployment("my-dep", namespace, label,
		corev1.Container{Name: containerName, Image: "nginx:v1"},
	)
	fakeClient := fake.NewSimpleClientset(dep)
	fakeClient.Fake.PrependReactor("update", "deployments",
		func(_ k8sTesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("persistent error")
		},
	)

	c := cache.NewGeneralCache[*corev1.Container]()
	c.Add(containerName, &corev1.Container{Name: containerName, Image: "nginx:v2"})
	w := newTestWorker(c, fakeClient)
	w.maxRetries = 2

	key := (&CRDWorkerEvent{Type: EventTypeUpdate, ContainerName: containerName, Namespace: namespace}).key()

	// 1st failure => retry.
	w.queue.Add(key)
	raw1, _ := w.queue.Get()
	w.process(raw1)
	if n := w.queue.NumRequeues(raw1); n != 1 {
		t.Fatalf("expected NumRequeues=1 after first failure, got %d", n)
	}

	// 2nd failure => retry (reaches configured cap count).
	raw2, _ := w.queue.Get()
	w.process(raw2)
	if n := w.queue.NumRequeues(raw2); n != 2 {
		t.Fatalf("expected NumRequeues=2 after second failure, got %d", n)
	}

	// 3rd failure => should drop and forget.
	raw3, _ := w.queue.Get()
	w.process(raw3)
	if n := w.queue.NumRequeues(raw3); n != 0 {
		t.Errorf("expected NumRequeues reset to 0 after drop, got %d", n)
	}
}

// --- TestCRDWorker_Run ---

func TestCRDWorker_Run_ShutdownOnContextCancel(t *testing.T) {
	c := cache.NewGeneralCache[*corev1.Container]()
	w := newTestWorker(c, fake.NewSimpleClientset())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx, 2)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Error("Run did not return after context cancellation")
	}
}

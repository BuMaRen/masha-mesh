package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewServiceController(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	ctrl := NewServiceController(clientset, "default")

	if ctrl == nil {
		t.Fatal("NewServiceController returned nil")
	}

	if ctrl.namespace != "default" {
		t.Errorf("expected namespace 'default', got '%s'", ctrl.namespace)
	}

	if ctrl.services == nil {
		t.Error("services map is nil")
	}
}

func TestSyncServices(t *testing.T) {
	// Create a fake service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.0.0.1",
			Ports: []corev1.ServicePort{
				{
					Port: 80,
				},
			},
		},
	}

	clientset := fake.NewSimpleClientset(service)
	ctrl := NewServiceController(clientset, "default")

	ctx := context.Background()
	err := ctrl.syncServices(ctx)
	if err != nil {
		t.Fatalf("syncServices failed: %v", err)
	}

	services := ctrl.GetServices()
	if len(services) != 1 {
		t.Errorf("expected 1 service, got %d", len(services))
	}

	key := "default/test-service"
	svc, ok := services[key]
	if !ok {
		t.Errorf("service %s not found", key)
	}

	if svc.Name != "test-service" {
		t.Errorf("expected service name 'test-service', got '%s'", svc.Name)
	}

	if svc.ClusterIP != "10.0.0.1" {
		t.Errorf("expected ClusterIP '10.0.0.1', got '%s'", svc.ClusterIP)
	}
}

func TestGetService(t *testing.T) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-service",
			Namespace: "kube-system",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.0.0.2",
			Ports: []corev1.ServicePort{
				{
					Port: 443,
				},
			},
		},
	}

	clientset := fake.NewSimpleClientset(service)
	ctrl := NewServiceController(clientset, "")

	ctx := context.Background()
	ctrl.syncServices(ctx)

	svc, ok := ctrl.GetService("kube-system", "my-service")
	if !ok {
		t.Fatal("service not found")
	}

	if svc.Name != "my-service" {
		t.Errorf("expected service name 'my-service', got '%s'", svc.Name)
	}

	if svc.Namespace != "kube-system" {
		t.Errorf("expected namespace 'kube-system', got '%s'", svc.Namespace)
	}
}

func TestSelectorMatches(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	ctrl := NewServiceController(clientset, "")

	tests := []struct {
		name     string
		selector map[string]string
		labels   map[string]string
		expected bool
	}{
		{
			name:     "exact match",
			selector: map[string]string{"app": "test"},
			labels:   map[string]string{"app": "test"},
			expected: true,
		},
		{
			name:     "no match",
			selector: map[string]string{"app": "test"},
			labels:   map[string]string{"app": "other"},
			expected: false,
		},
		{
			name:     "subset match",
			selector: map[string]string{"app": "test"},
			labels:   map[string]string{"app": "test", "version": "v1"},
			expected: true,
		},
		{
			name:     "empty selector",
			selector: map[string]string{},
			labels:   map[string]string{"app": "test"},
			expected: true,
		},
		{
			name:     "missing label",
			selector: map[string]string{"app": "test", "env": "prod"},
			labels:   map[string]string{"app": "test"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ctrl.selectorMatches(tt.selector, tt.labels)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

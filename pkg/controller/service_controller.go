package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// ServiceInfo represents discovered service information
type ServiceInfo struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	ClusterIP string            `json:"clusterIp"`
	Ports     []int32           `json:"ports"`
	Endpoints []string          `json:"endpoints"`
	Labels    map[string]string `json:"labels"`
}

// ServiceController watches Kubernetes services and pods
type ServiceController struct {
	clientset kubernetes.Interface
	namespace string
	services  map[string]*ServiceInfo
	mu        sync.RWMutex
}

// NewServiceController creates a new service controller
func NewServiceController(clientset kubernetes.Interface, namespace string) *ServiceController {
	return &ServiceController{
		clientset: clientset,
		namespace: namespace,
		services:  make(map[string]*ServiceInfo),
	}
}

// Run starts the controller
func (c *ServiceController) Run(ctx context.Context) error {
	klog.Info("Starting service controller...")

	// Initial sync
	if err := c.syncServices(ctx); err != nil {
		return fmt.Errorf("initial sync failed: %v", err)
	}

	// Watch for service changes
	go c.watchServices(ctx)

	// Watch for pod changes (to update endpoints)
	go c.watchPods(ctx)

	<-ctx.Done()
	klog.Info("Service controller stopped")
	return nil
}

// syncServices performs initial sync of services
func (c *ServiceController) syncServices(ctx context.Context) error {
	services, err := c.clientset.CoreV1().Services(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, svc := range services.Items {
		c.addService(&svc)
	}

	klog.Infof("Synced %d services", len(services.Items))
	return nil
}

// watchServices watches for service changes
func (c *ServiceController) watchServices(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		watcher, err := c.clientset.CoreV1().Services(c.namespace).Watch(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Errorf("Failed to watch services: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		c.handleServiceEvents(ctx, watcher)
		watcher.Stop()
	}
}

// handleServiceEvents processes service events
func (c *ServiceController) handleServiceEvents(ctx context.Context, watcher watch.Interface) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return
			}

			svc, ok := event.Object.(*corev1.Service)
			if !ok {
				continue
			}

			c.mu.Lock()
			switch event.Type {
			case watch.Added, watch.Modified:
				c.addService(svc)
				klog.Infof("Service %s/%s updated", svc.Namespace, svc.Name)
			case watch.Deleted:
				c.deleteService(svc)
				klog.Infof("Service %s/%s deleted", svc.Namespace, svc.Name)
			}
			c.mu.Unlock()
		}
	}
}

// watchPods watches for pod changes to update endpoints
func (c *ServiceController) watchPods(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		watcher, err := c.clientset.CoreV1().Pods(c.namespace).Watch(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Errorf("Failed to watch pods: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		c.handlePodEvents(ctx, watcher)
		watcher.Stop()
	}
}

// handlePodEvents processes pod events
func (c *ServiceController) handlePodEvents(ctx context.Context, watcher watch.Interface) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			// Update endpoints for affected services
			if event.Type == watch.Modified || event.Type == watch.Deleted {
				c.updateEndpointsForPod(ctx, pod)
			}
		}
	}
}

// updateEndpointsForPod updates endpoints for services matching the pod
func (c *ServiceController) updateEndpointsForPod(ctx context.Context, pod *corev1.Pod) {
	services, err := c.clientset.CoreV1().Services(pod.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Errorf("Failed to list services: %v", err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, svc := range services.Items {
		// Check if service selector matches pod labels
		if c.selectorMatches(svc.Spec.Selector, pod.Labels) {
			c.updateServiceEndpoints(ctx, &svc)
		}
	}
}

// selectorMatches checks if selector matches labels
func (c *ServiceController) selectorMatches(selector, labels map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// addService adds or updates a service
func (c *ServiceController) addService(svc *corev1.Service) {
	key := fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)

	ports := make([]int32, 0, len(svc.Spec.Ports))
	for _, port := range svc.Spec.Ports {
		ports = append(ports, port.Port)
	}

	info := &ServiceInfo{
		Name:      svc.Name,
		Namespace: svc.Namespace,
		ClusterIP: svc.Spec.ClusterIP,
		Ports:     ports,
		Labels:    svc.Labels,
		Endpoints: []string{},
	}

	c.services[key] = info
}

// updateServiceEndpoints updates endpoints for a service
func (c *ServiceController) updateServiceEndpoints(ctx context.Context, svc *corev1.Service) {
	key := fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)

	endpoints, err := c.clientset.CoreV1().Endpoints(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get endpoints for %s: %v", key, err)
		return
	}

	var endpointAddrs []string
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			endpointAddrs = append(endpointAddrs, addr.IP)
		}
	}

	if info, ok := c.services[key]; ok {
		info.Endpoints = endpointAddrs
	}
}

// deleteService removes a service
func (c *ServiceController) deleteService(svc *corev1.Service) {
	key := fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)
	delete(c.services, key)
}

// GetServices returns all discovered services
func (c *ServiceController) GetServices() map[string]*ServiceInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to avoid concurrent access issues
	result := make(map[string]*ServiceInfo, len(c.services))
	for k, v := range c.services {
		result[k] = v
	}
	return result
}

// GetService returns a specific service
func (c *ServiceController) GetService(namespace, name string) (*ServiceInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	info, ok := c.services[key]
	return info, ok
}

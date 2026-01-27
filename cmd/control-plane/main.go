package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"google.golang.org/grpc"

	pb "github.com/BuMaRen/masha-mesh/pkg/api"
)

// ServiceRegistry holds registered services and their endpoints
type ServiceRegistry struct {
	mu       sync.RWMutex
	services map[string][]string // service name -> list of endpoints
}

func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[string][]string),
	}
}

func (sr *ServiceRegistry) Register(serviceName string, endpoint string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	if _, exists := sr.services[serviceName]; !exists {
		sr.services[serviceName] = []string{}
	}
	
	// Check if endpoint already exists
	for _, ep := range sr.services[serviceName] {
		if ep == endpoint {
			return
		}
	}
	
	sr.services[serviceName] = append(sr.services[serviceName], endpoint)
	log.Printf("Registered service %s with endpoint %s", serviceName, endpoint)
}

func (sr *ServiceRegistry) GetEndpoints(serviceName string) []string {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	
	if endpoints, exists := sr.services[serviceName]; exists {
		return append([]string{}, endpoints...)
	}
	return []string{}
}

func (sr *ServiceRegistry) GetAllServices() map[string][]string {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	
	result := make(map[string][]string)
	for name, endpoints := range sr.services {
		result[name] = append([]string{}, endpoints...)
	}
	return result
}

// ControlPlaneServer implements the gRPC service for control plane
type ControlPlaneServer struct {
	pb.UnimplementedControlPlaneServer
	registry *ServiceRegistry
}

func (s *ControlPlaneServer) GetServiceConfig(ctx context.Context, req *pb.ServiceConfigRequest) (*pb.ServiceConfigResponse, error) {
	log.Printf("Received config request for service: %s", req.ServiceName)
	
	endpoints := s.registry.GetEndpoints(req.ServiceName)
	
	return &pb.ServiceConfigResponse{
		ServiceName: req.ServiceName,
		Endpoints:   endpoints,
		Protocol:    "http",
	}, nil
}

func (s *ControlPlaneServer) RegisterService(ctx context.Context, req *pb.ServiceRegistration) (*pb.RegistrationResponse, error) {
	log.Printf("Registering service: %s at %s", req.ServiceName, req.Endpoint)
	
	s.registry.Register(req.ServiceName, req.Endpoint)
	
	return &pb.RegistrationResponse{
		Success: true,
		Message: fmt.Sprintf("Service %s registered successfully", req.ServiceName),
	}, nil
}

func (s *ControlPlaneServer) StreamConfig(req *pb.ServiceConfigRequest, stream pb.ControlPlane_StreamConfigServer) error {
	log.Printf("Starting config stream for service: %s", req.ServiceName)
	
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
			endpoints := s.registry.GetEndpoints(req.ServiceName)
			resp := &pb.ServiceConfigResponse{
				ServiceName: req.ServiceName,
				Endpoints:   endpoints,
				Protocol:    "http",
			}
			if err := stream.Send(resp); err != nil {
				return err
			}
		}
	}
}

func startGRPCServer(registry *ServiceRegistry) {
	lis, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	
	grpcServer := grpc.NewServer()
	pb.RegisterControlPlaneServer(grpcServer, &ControlPlaneServer{
		registry: registry,
	})
	
	log.Println("Control Plane gRPC server listening on :9090")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func startHTTPServer(registry *ServiceRegistry) {
	http.HandleFunc("/services", func(w http.ResponseWriter, r *http.Request) {
		services := registry.GetAllServices()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(services)
	})
	
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	log.Println("Control Plane HTTP API listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

func main() {
	log.Println("Starting Control Plane...")
	
	registry := NewServiceRegistry()
	
	// Register some example services
	registry.Register("example-service", "localhost:9001")
	registry.Register("example-service", "localhost:9002")
	
	// Start gRPC server in a goroutine
	go startGRPCServer(registry)
	
	// Start HTTP server (blocks)
	startHTTPServer(registry)
}

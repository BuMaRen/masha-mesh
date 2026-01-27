package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/BuMaRen/masha-mesh/pkg/controller"
	"k8s.io/klog/v2"
)

// Server represents the HTTP API server
type Server struct {
	port       int
	controller *controller.ServiceController
	server     *http.Server
}

// NewServer creates a new HTTP server
func NewServer(port int, ctrl *controller.ServiceController) *Server {
	return &Server{
		port:       port,
		controller: ctrl,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/healthz", s.healthHandler)

	// Readiness check endpoint
	mux.HandleFunc("/readyz", s.readyHandler)

	// List all services
	mux.HandleFunc("/api/v1/services", s.listServicesHandler)

	// Get specific service
	mux.HandleFunc("/api/v1/services/", s.getServiceHandler)

	// Root endpoint with API info
	mux.HandleFunc("/", s.rootHandler)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.loggingMiddleware(mux),
	}

	klog.Infof("Starting HTTP server on port %d", s.port)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// loggingMiddleware logs all HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		klog.Infof("HTTP %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

// healthHandler handles health check requests
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

// readyHandler handles readiness check requests
func (s *Server) readyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ready",
	})
}

// rootHandler provides API information
func (s *Server) rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":    "masha-mesh control-face",
		"version": "v1",
		"endpoints": []string{
			"GET /healthz - Health check",
			"GET /readyz - Readiness check",
			"GET /api/v1/services - List all discovered services",
			"GET /api/v1/services/{namespace}/{name} - Get specific service",
		},
	})
}

// listServicesHandler lists all discovered services
func (s *Server) listServicesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	services := s.controller.GetServices()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"services": services,
		"count":    len(services),
	})
}

// getServiceHandler gets a specific service
func (s *Server) getServiceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse namespace and name from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/services/")
	parts := strings.Split(path, "/")

	if len(parts) != 2 {
		http.Error(w, "Invalid path. Use /api/v1/services/{namespace}/{name}", http.StatusBadRequest)
		return
	}

	namespace, name := parts[0], parts[1]

	service, ok := s.controller.GetService(namespace, name)
	if !ok {
		http.Error(w, fmt.Sprintf("Service %s/%s not found", namespace, name), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(service)
}

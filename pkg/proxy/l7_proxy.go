package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

// L7Proxy implements an HTTP Layer 7 proxy with load balancing
type L7Proxy struct {
	listenAddr string
	mu         sync.RWMutex
	backends   []*url.URL
	current    int
}

// NewL7Proxy creates a new L7 HTTP proxy
func NewL7Proxy(listenAddr string, backends []string) (*L7Proxy, error) {
	backendURLs := make([]*url.URL, 0, len(backends))
	for _, backend := range backends {
		u, err := url.Parse(backend)
		if err != nil {
			return nil, fmt.Errorf("invalid backend URL %s: %v", backend, err)
		}
		backendURLs = append(backendURLs, u)
	}
	
	return &L7Proxy{
		listenAddr: listenAddr,
		backends:   backendURLs,
		current:    0,
	}, nil
}

// UpdateBackends updates the list of backend servers
func (p *L7Proxy) UpdateBackends(backends []string) error {
	backendURLs := make([]*url.URL, 0, len(backends))
	for _, backend := range backends {
		u, err := url.Parse(backend)
		if err != nil {
			return fmt.Errorf("invalid backend URL %s: %v", backend, err)
		}
		backendURLs = append(backendURLs, u)
	}
	
	p.mu.Lock()
	defer p.mu.Unlock()
	p.backends = backendURLs
	p.current = 0
	
	log.Printf("L7 Proxy: Updated backends to %v", backends)
	return nil
}

// Start starts the L7 proxy server
func (p *L7Proxy) Start() error {
	handler := http.HandlerFunc(p.handleRequest)
	
	log.Printf("L7 HTTP Proxy listening on %s", p.listenAddr)
	return http.ListenAndServe(p.listenAddr, handler)
}

func (p *L7Proxy) handleRequest(w http.ResponseWriter, r *http.Request) {
	backend := p.getNextBackend()
	if backend == nil {
		http.Error(w, "No backends available", http.StatusServiceUnavailable)
		return
	}
	
	log.Printf("L7 Proxy: %s %s -> %s", r.Method, r.URL.Path, backend.String())
	
	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(backend)
	
	// Modify request
	r.URL.Host = backend.Host
	r.URL.Scheme = backend.Scheme
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Host = backend.Host
	
	// Add custom error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("L7 Proxy: Error forwarding to %s: %v", backend.String(), err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}
	
	// Proxy the request
	proxy.ServeHTTP(w, r)
}

func (p *L7Proxy) getNextBackend() *url.URL {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if len(p.backends) == 0 {
		return nil
	}
	
	// Round-robin load balancing
	backend := p.backends[p.current]
	p.current = (p.current + 1) % len(p.backends)
	
	return backend
}

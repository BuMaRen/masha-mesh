package l7

import (
	"net/http"
	"net/http/httputil"

	"k8s.io/klog/v2"
)

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Create reverse proxy
	// Director 留空是因为实际的目标地址改写在 RoundTrip 中完成
	proxy := &httputil.ReverseProxy{
		Transport: s,
		Director:  func(req *http.Request) {},
	}

	// Add custom error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		klog.Errorf("L7 Proxy: Error forwarding request: %v", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}

	// Proxy the request
	proxy.ServeHTTP(w, r)
}

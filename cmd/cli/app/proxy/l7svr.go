package proxy

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/http/httputil"

	"github.com/BuMaRen/mesh/pkg/cli"
)

type L7OptionsFunc func(*L7Proxy)

type L7Proxy struct {
	address    string
	meshClient *cli.MeshClient
}

func NewL7RouteServer(meshClient *cli.MeshClient, address string, opts ...L7OptionsFunc) *L7Proxy {
	l7svr := &L7Proxy{
		meshClient: meshClient,
		address:    address,
	}
	for _, opt := range opts {
		opt(l7svr)
	}
	return l7svr
}

func (l7 *L7Proxy) Complete() {
	_, _, err := net.SplitHostPort(l7.address)
	if err != nil {
		panic(err)
	}
}

// Run 启动 L7 代理服务器，阻塞监听 HTTP 请求并进行反向代理
func (l7 *L7Proxy) Run(ctx context.Context) error {
	handler := http.HandlerFunc(l7.handleRequest)
	return http.ListenAndServe(l7.address, handler)
}

func (l7 *L7Proxy) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Create reverse proxy
	proxy := &httputil.ReverseProxy{Transport: l7}

	// Add custom error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("L7 Proxy: Error forwarding: %v", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}

	// Proxy the request
	proxy.ServeHTTP(w, r)
}

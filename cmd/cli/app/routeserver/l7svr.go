package routeserver

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/BuMaRen/mesh/pkg/cli"
)

type L7OptionsFunc func(*L7RouteServer)

type L7RouteServer struct {
	address    string
	meshClient *cli.MeshClient
}

func NewL7RouteServer(meshClient *cli.MeshClient, opts ...L7OptionsFunc) *L7RouteServer {
	l7svr := &L7RouteServer{
		meshClient: meshClient,
	}
	for _, opt := range opts {
		opt(l7svr)
	}
	return l7svr
}

func (l7 *L7RouteServer) Complete() {
	_, _, err := net.SplitHostPort(l7.address)
	if err != nil {
		panic(err)
	}
}

func (l7 *L7RouteServer) Run() error {
	handler := http.HandlerFunc(l7.handleRequest)
	return http.ListenAndServe(l7.address, handler)
}

func (l7 *L7RouteServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	backend := &url.URL{}
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

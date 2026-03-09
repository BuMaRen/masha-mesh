package routeserver

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"

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

// 判断host是不是由service组成，是的话返回serviceName和True
func serviceAsHost(host string) (string, string, bool) {
	hostStr := host
	port := ""
	if strings.Contains(host, ":") {
		parts := strings.Split(host, ":")
		hostStr = parts[0]
		port = parts[1]
	}

	return hostStr, port, net.ParseIP(hostStr) == nil
}

func (l7 *L7RouteServer) availableEndpoints(serviceName string) []string {
	result := []string{}
	eps := l7.meshClient.GetServiceIps(serviceName)
	for _, ep := range eps {
		result = append(result, ep[0])
	}
	return result
}

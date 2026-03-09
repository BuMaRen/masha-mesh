package routeserver

import (
	"github.com/BuMaRen/mesh/pkg/cli"
)

type OptionsFunc func(*ProxyOptions)

func WithL4address(address string) OptionsFunc {
	return func(opts *ProxyOptions) {
		opts.L4Address = address
	}
}

func WithL7Address(address string) OptionsFunc {
	return func(opts *ProxyOptions) {
		opts.L7Address = address
	}
}

func WithL7Port(port int) OptionsFunc {
	return func(opts *ProxyOptions) {
		opts.L7Port = port
	}
}

type ProxyOptions struct {
	L4Address string
	L7Address string
	L7Port    int
}

func NewProxyOptions(opts ...OptionsFunc) *ProxyOptions {
	options := &ProxyOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

func (o *ProxyOptions) Complete(meshClient *cli.MeshClient) *RouteServer {
	return &RouteServer{
		l4Proxy: NewL4RouteServer(o.L4Address),
		l7Proxy: NewL7RouteServer(meshClient, o.L7Address),
	}
}

type RouteServer struct {
	l4Proxy *L4RouteServer
	l7Proxy *L7RouteServer
}

// TODO: 需要增加优雅关闭的逻辑
func (s *RouteServer) Run() error {
	go s.l4Proxy.ProxyLoop()
	return s.l7Proxy.Run()
}

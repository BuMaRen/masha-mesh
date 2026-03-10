package proxy

import (
	"context"

	"github.com/BuMaRen/mesh/pkg/cli"
)

type OptionsFunc func(*ProxyOptions)

func WithL4address(address string) OptionsFunc {
	return func(opts *ProxyOptions) {
		opts.l4Address = address
	}
}

func WithL7Address(address string) OptionsFunc {
	return func(opts *ProxyOptions) {
		opts.l7Address = address
	}
}

func WithL7Port(port int) OptionsFunc {
	return func(opts *ProxyOptions) {
		opts.l7Port = port
	}
}

type ProxyOptions struct {
	l4Address string
	l7Address string
	l7Port    int
}

func NewProxyOptions(opts ...OptionsFunc) *ProxyOptions {
	options := &ProxyOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

func (o *ProxyOptions) Complete(meshClient *cli.MeshClient) *Proxy {
	return &Proxy{
		l4Proxy: NewL4RouteServer(o.l4Address, o.l7Port),
		l7Proxy: NewL7RouteServer(meshClient, o.l7Address),
	}
}

type Proxy struct {
	l4Proxy *L4Proxy
	l7Proxy *L7Proxy
}

// TODO: 需要增加优雅关闭的逻辑
func (s *Proxy) Run(ctx context.Context) error {
	go s.l4Proxy.ProxyLoop(ctx)
	return s.l7Proxy.Run(ctx)
}

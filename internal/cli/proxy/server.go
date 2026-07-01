package proxy

import (
	"context"

	"github.com/BuMaRen/mesh/internal/cli/breaker"
	"github.com/BuMaRen/mesh/internal/cli/rpcclient"
	"k8s.io/klog/v2"
)

type Proxy struct {
	listener *Listener
}

func NewProxy(meshClient *rpcclient.MeshClient, opts *Options) (*Proxy, error) {
	if err := opts.LoadConfig(); err != nil {
		return nil, err
	}

	var brk *breaker.Breaker
	if opts.Config() != nil && opts.Config().HTTP.Enabled {
		brk = breaker.NewBreaker(opts.BreakerOptions())
	}

	listener := NewListener(meshClient, opts.Config(), brk)

	return &Proxy{
		listener: listener,
	}, nil
}

func (p *Proxy) Run(ctx context.Context, opts *Options) error {
	klog.Infof("[Proxy] starting on %s", opts.ListenAddress())
	return p.listener.Listen(ctx, opts.ListenAddress())
}

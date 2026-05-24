package proxy

import (
	"context"

	"github.com/BuMaRen/mesh/internal/cli/proxy/l4"
	"github.com/BuMaRen/mesh/internal/cli/proxy/l7"
	"github.com/BuMaRen/mesh/internal/cli/rpcclient"
	"k8s.io/klog/v2"
)

type Proxy struct {
	svrL4 *l4.Server
	svrL7 *l7.Server
}

func NewProxy(meshClient *rpcclient.MeshClient) *Proxy {
	return &Proxy{
		svrL4: l4.NewServer(),
		svrL7: l7.NewServer(meshClient),
	}
}

func (p *Proxy) Run(ctx context.Context, opts *Options) {
	go func() {
		if err := p.svrL4.Run(ctx, opts.L4Options()); err != nil {
			klog.Errorf("l4 proxy loop failed with error: %+v", err)
		}
	}()
	if err := p.svrL7.Run(ctx, opts.L7Options()); err != nil {
		klog.Errorf("l7 proxy run failed with error: %+v", err)
	}
}

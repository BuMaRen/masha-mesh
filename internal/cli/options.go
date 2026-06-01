package cli

import (
	"github.com/BuMaRen/mesh/internal/cli/breaker"
	"github.com/BuMaRen/mesh/internal/cli/proxy"
	"github.com/BuMaRen/mesh/internal/cli/rpcclient"
	"github.com/BuMaRen/mesh/internal/cli/stgsvr"
	"github.com/spf13/cobra"
)

type Options struct {
	stgSvrOptions    *stgsvr.Options
	breakerOptions   *breaker.Options
	proxyOptions     *proxy.Options
	rpcClientOptions *rpcclient.Options
}

func NewOptions() *Options {
	return &Options{
		stgSvrOptions:    stgsvr.NewOptions(),
		breakerOptions:   breaker.NewOptions(),
		proxyOptions:     proxy.NewOptions(),
		rpcClientOptions: rpcclient.NewOptions(),
	}
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	o.stgSvrOptions.AddFlags(cmd)
	o.breakerOptions.AddFlags(cmd)
	o.proxyOptions.AddFlags(cmd)
	o.rpcClientOptions.AddFlags(cmd)
}

func (o *Options) StgSvrOptions() *stgsvr.Options {
	return o.stgSvrOptions
}

func (o *Options) BreakerOptions() *breaker.Options {
	return o.breakerOptions
}

func (o *Options) ProxyOptions() *proxy.Options {
	return o.proxyOptions
}

func (o *Options) RPCClientOptions() *rpcclient.Options {
	return o.rpcClientOptions
}

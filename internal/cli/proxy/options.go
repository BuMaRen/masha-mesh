package proxy

import (
	"github.com/BuMaRen/mesh/internal/cli/breaker"
	"github.com/BuMaRen/mesh/internal/cli/proxy/l4"
	"github.com/BuMaRen/mesh/internal/cli/proxy/l7"
	"github.com/spf13/cobra"
)

type Options struct {
	l4Opts      *l4.Options
	l7Opts      *l7.Options
	breakerOpts *breaker.Options
}

func NewOptions() *Options {
	return &Options{
		l4Opts:      l4.NewOptions(),
		l7Opts:      l7.NewOptions(),
		breakerOpts: breaker.NewOptions(),
	}
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	o.l4Opts.AddFlags(cmd)
	o.l7Opts.AddFlags(cmd)
	o.breakerOpts.AddFlags(cmd)
	o.l7Opts.SetBreakerOpts(o.breakerOpts)
}

func (o *Options) L4Options() *l4.Options {
	return o.l4Opts
}

func (o *Options) L7Options() *l7.Options {
	return o.l7Opts
}

func (o *Options) BreakerOptions() *breaker.Options {
	return o.breakerOpts
}

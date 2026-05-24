package l7

import (
	"github.com/BuMaRen/mesh/internal/cli/breaker"
	"github.com/spf13/cobra"
)

type Options struct {
	l7Address   string
	breakerOpts *breaker.Options
}

func NewOptions() *Options {
	return &Options{
		l7Address:   ":8080",
		breakerOpts: breaker.NewOptions(),
	}
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.l7Address, "l7-address", o.l7Address, "L7 proxy listen address")
	o.breakerOpts.AddFlags(cmd)
}

func (o *Options) Address() string {
	return o.l7Address
}

func (o *Options) SetBreakerOpts(opts *breaker.Options) {
	o.breakerOpts = opts
}

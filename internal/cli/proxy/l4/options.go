package l4

import "github.com/spf13/cobra"

type Options struct {
	l4Address    string
	dstL7Address string
}

func NewOptions() *Options {
	return &Options{
		l4Address:    ":8081",
		dstL7Address: ":8080",
	}
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.l4Address, "l4-address", ":8081", "L4 proxy listen address")
	cmd.Flags().StringVar(&o.dstL7Address, "dst-l7-address", ":8080", "L7 proxy destination address")
}

func (o *Options) Address() string {
	return o.l4Address
}

func (o *Options) DstL7Address() string {
	return o.dstL7Address
}

package stgsvr

import "github.com/spf13/cobra"

type Options struct {
	stgSvrAddress string
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.stgSvrAddress, "stg-svr-address", ":8081", "Address for staging server to listen on")
}

package app

import "github.com/spf13/cobra"

type Options struct {
	crt     string
	key     string
	address string
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) AddFlags(command *cobra.Command) {
	command.Flags().StringVar(&o.address, "address", ":8443", "The address to listen on for HTTPS requests.")
	command.Flags().StringVar(&o.crt, "crt", "", "The path to the TLS certificate file.")
	command.Flags().StringVar(&o.key, "key", "", "The path to the TLS key file.")
	command.MarkFlagRequired("crt")
	command.MarkFlagRequired("key")
}

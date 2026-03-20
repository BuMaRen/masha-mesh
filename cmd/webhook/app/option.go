package app

import "github.com/spf13/cobra"

type Options struct {
	crt string
	key string
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) AddFlags(command *cobra.Command) {
	command.Flags().StringVar(&o.crt, "crt", "", "The path to the TLS certificate file.")
	command.Flags().StringVar(&o.key, "key", "", "The path to the TLS key file.")
}

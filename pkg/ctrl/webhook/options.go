package webhook

import (
	"github.com/spf13/cobra"
)

type Options struct {
	certFile string
	keyFile  string
	address  string
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.certFile, "cert-file", "", "Path to the TLS certificate file")
	cmd.Flags().StringVar(&o.keyFile, "key-file", "", "Path to the TLS key file")
	cmd.Flags().StringVar(&o.address, "webhook-address", ":9090", "The address the webhook server binds to")
}

func NewOptions() *Options {
	return &Options{}
}

func InitializeOptions(certFile, keyFile, address string) *Options {
	return &Options{
		certFile: certFile,
		keyFile:  keyFile,
		address:  address,
	}
}

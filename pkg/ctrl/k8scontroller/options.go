package k8scontroller

import "github.com/spf13/cobra"

type Options struct {
	customResource string
	address        string
	certFile       string
	keyFile        string
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.customResource, "custom-resource", "myresource", "Name of the custom resource to watch")
	cmd.Flags().StringVar(&o.address, "address", ":8080", "Address to listen on (e.g., :8080)")
	cmd.Flags().StringVar(&o.certFile, "tls-cert-file", "", "Path to TLS certificate file")
	cmd.Flags().StringVar(&o.keyFile, "tls-key-file", "", "Path to TLS key file")
	cmd.MarkFlagRequired("tls-cert-file")
	cmd.MarkFlagRequired("tls-key-file")
}

func NewOptions() *Options {
	return &Options{}
}

func InitializeOptions(customResource, address, certFile, keyFile string) *Options {
	return &Options{
		customResource: customResource,
		address:        address,
		certFile:       certFile,
		keyFile:        keyFile,
	}
}

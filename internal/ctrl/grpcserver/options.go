package grpcserver

import "github.com/spf13/cobra"

type Options struct {
	network string
	address string
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.network, "grpc-network", "tcp", "Network type to listen on (e.g., tcp)")
	cmd.Flags().StringVar(&o.address, "grpc-address", ":50051", "Address to listen on (e.g., :50051)")
}

func NewOptions() *Options {
	return &Options{}
}

func InitializeOptions(network, address string) *Options {
	return &Options{
		network: network,
		address: address,
	}
}

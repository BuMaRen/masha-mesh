package rpcclient

import "github.com/spf13/cobra"

type Options struct {
	uid      string // unique identifier for the client instance
	capacity int    // service cache capacity
	remote   string // target service in format namespace/name
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.uid, "uid", "mesh-cli", "Unique identifier for the client")
	cmd.Flags().IntVar(&o.capacity, "cache-capacity", 100, "Capacity of the service cache")
	cmd.Flags().StringVar(&o.remote, "target", "mesh-ctrl:50051", "Target service in format namespace/name")
}

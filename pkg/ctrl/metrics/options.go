package metrics

import "github.com/spf13/cobra"

type Options struct {
	address string
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	// 这里可以添加一些命令行参数，例如：
	cmd.Flags().StringVar(&o.address, "metrics-address", ":8080", "Address to serve metrics on")
}

func NewMetricsOptions() *Options {
	return &Options{}
}

package ctrl

import (
	"github.com/BuMaRen/mesh/pkg/ctrl/grpcserver"
	"github.com/BuMaRen/mesh/pkg/ctrl/metrics"
	"github.com/spf13/cobra"
)

type Options struct {
	gvrVersion     string
	gvrGroup       string
	gvrResource    string
	grpcOptions    *grpcserver.Options
	metricsOptions *metrics.Options
}

func NewOptions() *Options {
	return &Options{
		grpcOptions:    grpcserver.NewOptions(),
		metricsOptions: metrics.NewMetricsOptions(),
	}
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.gvrVersion, "gvr-version", "v1", "Version of the GVR to watch")
	cmd.Flags().StringVar(&o.gvrGroup, "gvr-group", "masha.io", "Group of the GVR to watch")
	cmd.Flags().StringVar(&o.gvrResource, "gvr-resource", "injections", "Resource of the GVR to watch")
	o.grpcOptions.AddFlags(cmd)
	o.metricsOptions.AddFlags(cmd)
}

func (o *Options) CtrlOptions() *grpcserver.Options {
	return o.grpcOptions
}

func (o *Options) MetricsOptions() *metrics.Options {
	return o.metricsOptions
}

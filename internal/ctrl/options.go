package ctrl

import (
	"github.com/BuMaRen/mesh/internal/ctrl/grpcserver"
	"github.com/BuMaRen/mesh/internal/ctrl/webhook"
	"github.com/BuMaRen/mesh/pkg/metrics"
	"github.com/spf13/cobra"
)

type Options struct {
	label          string
	gvrVersion     string
	gvrGroup       string
	gvrResource    string
	grpcOptions    *grpcserver.Options
	metricsOptions *metrics.Options
	webhookOptions *webhook.Options
}

func NewOptions() *Options {
	return &Options{
		grpcOptions:    grpcserver.NewOptions(),
		metricsOptions: metrics.NewMetricsOptions(),
		webhookOptions: webhook.NewOptions(),
	}
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	// label 指定 pod 中的 label key，其值用于确定需要注入的容器名称
	cmd.Flags().StringVar(&o.label, "label", "masha.io/injection", "Label to select workloads for injection")
	cmd.Flags().StringVar(&o.gvrVersion, "gvr-version", "v1", "Version of the GVR to watch")
	cmd.Flags().StringVar(&o.gvrGroup, "gvr-group", "masha.io", "Group of the GVR to watch")
	cmd.Flags().StringVar(&o.gvrResource, "gvr-resource", "containers", "Resource of the GVR to watch")
	o.grpcOptions.AddFlags(cmd)
	o.metricsOptions.AddFlags(cmd)
	o.webhookOptions.AddFlags(cmd)
}

func (o *Options) WebhookOptions() *webhook.Options {
	return o.webhookOptions
}

func (o *Options) CtrlOptions() *grpcserver.Options {
	return o.grpcOptions
}

func (o *Options) MetricsOptions() *metrics.Options {
	return o.metricsOptions
}

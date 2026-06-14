package ctrl

import (
	"github.com/BuMaRen/mesh/internal/ctrl/grpcserver"
	"github.com/spf13/cobra"
)

type StartUpOptions struct {
	label                   string
	gvrVersion              string
	gvrGroup                string
	gvrResource             string
	grpcOptions             *grpcserver.Options
	certFile                string
	keyFile                 string
	address                 string
	gracefulShutdownTimeout int
}

func NewStartUpOptions() *StartUpOptions {
	return &StartUpOptions{
		grpcOptions: grpcserver.NewOptions(),
	}
}

func (o *StartUpOptions) AddFlags(cmd *cobra.Command) {
	// label 指定 pod 中的 label key，其值用于确定需要注入的容器名称
	cmd.Flags().StringVar(&o.label, "label", "masha.io/injection", "Label to select workloads for injection")
	cmd.Flags().StringVar(&o.gvrVersion, "gvr-version", "v1", "Version of the GVR to watch")
	cmd.Flags().StringVar(&o.gvrGroup, "gvr-group", "masha.io", "Group of the GVR to watch")
	cmd.Flags().StringVar(&o.gvrResource, "gvr-resource", "containers", "Resource of the GVR to watch")
	cmd.Flags().StringVar(&o.certFile, "cert-file", "", "File containing the certificate for TLS")
	cmd.Flags().StringVar(&o.keyFile, "key-file", "", "File containing the key for TLS")
	cmd.Flags().StringVar(&o.address, "address", ":443", "Address to listen on")
	cmd.Flags().IntVar(&o.gracefulShutdownTimeout, "graceful-shutdown-timeout", 10, "Timeout for graceful shutdown")
	o.grpcOptions.AddFlags(cmd)
}

func (o *StartUpOptions) GrpcOptions() *grpcserver.Options {
	return o.grpcOptions
}

type ShutdownOptions struct {
	certFile                string
	address                 string
	gracefulShutdownTimeout int
}

func NewShutdownOptions() *ShutdownOptions {
	return &ShutdownOptions{}
}

func (o *ShutdownOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.certFile, "cert-file", "", "File containing the certificate for TLS")
	cmd.Flags().StringVar(&o.address, "address", ":443", "Address to listen on")
	cmd.Flags().IntVar(&o.gracefulShutdownTimeout, "graceful-shutdown-timeout", 10, "Timeout for graceful shutdown")
}

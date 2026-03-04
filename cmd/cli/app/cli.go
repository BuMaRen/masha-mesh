package app

import (
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	opts := NewOptions()
	rootCmd := &cobra.Command{
		Use:   "mesh-cli",
		Short: "A brief description of your application",
		Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
		// Uncomment the following line if your bare application
		// has an action associated with it:
		Run: func(cmd *cobra.Command, args []string) {
			newRpcClient(opts.target, opts.uid, opts.svcName)
		},
	}
	rootCmd.PersistentFlags().StringVar(&opts.target, "target", "mesh-ctrl:50051", "gRPC server target")
	rootCmd.PersistentFlags().StringVar(&opts.uid, "sidecar-id", "mesh-sidecar", "Sidecar ID to subscribe to")
	rootCmd.PersistentFlags().StringVar(&opts.svcName, "service-name", "mesh-ctrl", "Service name to subscribe to")
	return rootCmd
}

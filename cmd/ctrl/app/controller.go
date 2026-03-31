package app

import (
	"github.com/BuMaRen/mesh/pkg/ctrl/logic"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	opts := logic.NewOptions()
	rootCmd := &cobra.Command{
		Use:   "mesh-ctrl",
		Short: "A brief description of your application",
		Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
		// Uncomment the following line if your bare application
		// has an action associated with it:
		RunE: func(cmd *cobra.Command, args []string) error {
			svr := &logic.Logic{}
			if err := svr.Compelete(opts); err != nil {
				return err
			}
			return Serve(svr)
		},
	}

	rootCmd.Flags().IntVarP(&opts.GrpcPort, "port", "p", 50051, "grpc server port")
	rootCmd.Flags().StringVarP(&opts.Crt, "crt", "", "", "https server crt file")
	rootCmd.Flags().StringVarP(&opts.Key, "key", "", "", "https server key file")
	rootCmd.Flags().StringVarP(&opts.Address, "address", "a", "0.0.0.0:8443", "https server address")
	rootCmd.Flags().IntVarP(&opts.MapInitialSize, "map-initial-size", "", 1024, "initial size of the map")
	rootCmd.Flags().StringVarP(&opts.InjectionImageTag, "injection-image-tag", "", "hjmasha/mesh-cli:v0.1.53", "sidecar injection image tag")
	rootCmd.Flags().StringVarP(&opts.InjectionCommand, "injection-command", "", "/app/mesh-cli", "sidecar injection command")
	rootCmd.MarkFlagRequired("crt")
	rootCmd.MarkFlagRequired("key")
	return rootCmd
}

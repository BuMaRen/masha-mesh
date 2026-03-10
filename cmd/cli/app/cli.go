package app

import (
	"context"

	"github.com/BuMaRen/mesh/pkg/cli"
	"github.com/spf13/cobra"
)

func rootContext() context.Context {
	return context.Background()
}

func NewCommand() *cobra.Command {
	opts := &Options{}
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
			svcCache := cli.NewServiceCache(opts.cacheCapacity)
			meshClient := cli.NewMeshClient(opts.uid, svcCache)
			proxyServer, httpServer := opts.Complete(meshClient, cli.NewServiceContext())
			ctx := rootContext()
			proxyServer.Run(ctx)
			httpServer.Run(ctx)
		},
	}
	opts.ParseFlags(rootCmd)
	return rootCmd
}

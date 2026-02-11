package main

import (
	"context"
	"fmt"

	"github.com/BuMaRen/mesh/pkg/api/mesh"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func newRpcClient(target, sidecarID, serviceName string) {
	c, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	client := mesh.NewMeshCtrlClient(c)
	stream, err := client.Subscribe(context.Background(), &mesh.SubscriptionRequest{
		SidecarId:   sidecarID,
		ServiceName: serviceName,
	})
	if err != nil {
		panic(err)
	}
	for {
		event, err := stream.Recv()
		if err != nil {
			panic(err)
		}
		fmt.Printf("Received event: %+v\n", event)
	}
}

func newCommand() *cobra.Command {
	var target, sidecarID, serviceName string
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
			newRpcClient(target, sidecarID, serviceName)
		},
	}
	rootCmd.PersistentFlags().StringVar(&target, "target", "mesh-ctrl:50051", "gRPC server target")
	rootCmd.PersistentFlags().StringVar(&sidecarID, "sidecar-id", "mesh-sidecar", "Sidecar ID to subscribe to")
	rootCmd.PersistentFlags().StringVar(&serviceName, "service-name", "mesh-ctrl", "Service name to subscribe to")
	return rootCmd
}

func main() {
	command := newCommand()
	if err := command.Execute(); err != nil {
		panic(err)
	}
}

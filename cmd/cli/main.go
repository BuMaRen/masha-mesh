package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/BuMaRen/mesh/pkg/api/mesh"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var serverAddr string

	rootCmd := &cobra.Command{
		Use:   "mesh-cli",
		Short: "Mesh CLI client for service mesh control plane",
		Long:  "A command-line client to interact with the mesh control plane via gRPC",
	}

	rootCmd.PersistentFlags().StringVarP(&serverAddr, "server", "s", "localhost:50051", "Control plane server address")

	rootCmd.AddCommand(newSubscribeCommand(&serverAddr))
	rootCmd.AddCommand(newListCommand(&serverAddr))
	rootCmd.AddCommand(newUnsubscribeCommand(&serverAddr))

	return rootCmd
}

func newSubscribeCommand(serverAddr *string) *cobra.Command {
	var instanceID string
	var serviceName string

	cmd := &cobra.Command{
		Use:   "subscribe",
		Short: "Subscribe to service updates",
		Long:  "Subscribe to real-time updates for a specific service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return subscribeToService(*serverAddr, instanceID, serviceName)
		},
	}

	cmd.Flags().StringVarP(&instanceID, "instance", "i", "", "Instance ID (required)")
	cmd.Flags().StringVarP(&serviceName, "service", "n", "", "Service name (required)")
	cmd.MarkFlagRequired("instance")
	cmd.MarkFlagRequired("service")

	return cmd
}

func newListCommand(serverAddr *string) *cobra.Command {
	var instanceID string
	var serviceName string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List service endpoints",
		Long:  "Query current endpoints for a specific service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listService(*serverAddr, instanceID, serviceName)
		},
	}

	cmd.Flags().StringVarP(&instanceID, "instance", "i", "", "Instance ID (required)")
	cmd.Flags().StringVarP(&serviceName, "service", "n", "", "Service name (required)")
	cmd.MarkFlagRequired("instance")
	cmd.MarkFlagRequired("service")

	return cmd
}

func newUnsubscribeCommand(serverAddr *string) *cobra.Command {
	var instanceID string
	var serviceName string

	cmd := &cobra.Command{
		Use:   "unsubscribe",
		Short: "Unsubscribe from service updates",
		Long:  "Stop receiving updates for a specific service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return unsubscribeFromService(*serverAddr, instanceID, serviceName)
		},
	}

	cmd.Flags().StringVarP(&instanceID, "instance", "i", "", "Instance ID (required)")
	cmd.Flags().StringVarP(&serviceName, "service", "n", "", "Service name (required)")
	cmd.MarkFlagRequired("instance")
	cmd.MarkFlagRequired("service")

	return cmd
}

// subscribeToService 订阅服务更新（流式响应）
func subscribeToService(serverAddr, instanceID, serviceName string) error {
	conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	client := mesh.NewControlFaceClient(conn)
	ctx := context.Background()

	req := &mesh.SubscribeRequest{
		InstanceId:  instanceID,
		ServiceName: serviceName,
	}

	log.Printf("Subscribing to service '%s' with instance ID '%s'...\n", serviceName, instanceID)
	stream, err := client.Subscribe(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	log.Printf("Connected to %s, waiting for updates...", serverAddr)
	fmt.Println("---")

	for {
		log.Printf("Waiting for next update...")
		update, err := stream.Recv()
		if err == io.EOF {
			log.Println("Stream closed by server")
			break
		}
		if err != nil {
			log.Printf("error receiving update: %v", err)
			return fmt.Errorf("error receiving update: %w", err)
		}
		log.Printf("Received update: op=%s revision=%d endpoints=%d", update.OpType, update.Revision, len(update.Endpoints))

		printServiceUpdate(update)
	}

	return nil
}

// listService 查询服务当前状态
func listService(serverAddr, instanceID, serviceName string) error {
	conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	client := mesh.NewControlFaceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &mesh.SubscribeRequest{
		InstanceId:  instanceID,
		ServiceName: serviceName,
	}

	log.Printf("Listing endpoints for service '%s'...\n", serviceName)
	update, err := client.ListService(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to list service: %w", err)
	}

	printServiceUpdate(update)
	return nil
}

// unsubscribeFromService 取消订阅
func unsubscribeFromService(serverAddr, instanceID, serviceName string) error {
	conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	client := mesh.NewControlFaceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &mesh.SubscribeRequest{
		InstanceId:  instanceID,
		ServiceName: serviceName,
	}

	log.Printf("Unsubscribing from service '%s'...\n", serviceName)
	update, err := client.Unsubsribe(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe: %w", err)
	}

	printServiceUpdate(update)
	return nil
}

// printServiceUpdate 格式化输出服务更新信息
func printServiceUpdate(update *mesh.ServiceUpdate) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] Service Update:\n", timestamp)
	fmt.Printf("  Operation Type: %s\n", update.OpType)
	fmt.Printf("  Revision: %d\n", update.Revision)
	fmt.Printf("  Endpoints (%d):\n", len(update.Endpoints))

	if len(update.Endpoints) == 0 {
		fmt.Println("    (none)")
	} else {
		for uid, endpoint := range update.Endpoints {
			fmt.Printf("    - UID: %s\n", uid)
			fmt.Printf("      IP: %s\n", endpoint.Ip)
			fmt.Printf("      Port: %d\n", endpoint.Port)
		}
	}
	fmt.Println("---")
}

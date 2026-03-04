package app

import (
	"context"
	"fmt"
	"io"

	"github.com/BuMaRen/mesh/pkg/api/mesh"
	"github.com/BuMaRen/mesh/pkg/cli"
	"k8s.io/klog/v2"
)

func newRpcClient(remote, sidecarID, serviceName string) {
	klog.Info("Creating gRPC client with target: ", remote)
	client := cli.NewRpcClient(remote)
	stream, err := client.Subscribe(context.Background(), &mesh.SubscriptionRequest{
		SidecarId:   sidecarID,
		ServiceName: serviceName,
	})
	if err != nil {
		klog.Error("Failed to subscribe to events: ", err)
		panic(err)
	}
	klog.Info("Subscribed to events successfully, waiting for events...")
	for {
		event, err := stream.Recv()
		if err == io.EOF {
			klog.Info("Stream closed by server")
			break
		}
		if err != nil {
			klog.Error("Failed to receive event: ", err)
			panic(err)
		}
		fmt.Printf("Received event: %+v\n", event)
		switch event.GetOpType() {
		case mesh.OpType_ADDED:
			klog.Info("Receive a service-add event\n")
		case mesh.OpType_MODIFIED:
			klog.Info("Receive a service-modify event\n")
		case mesh.OpType_DELETED:
			klog.Info("Receive a service-delete event\n")
		}
	}
}

package cli

import (
	"context"

	"github.com/BuMaRen/mesh/pkg/api/mesh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog/v2"
)

func NewRpcClient(remote string) mesh.MeshCtrlClient {
	c, err := grpc.NewClient(remote, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		klog.Error("Failed to create gRPC client: ", err)
		panic(err)
	}
	return mesh.NewMeshCtrlClient(c)
}

type MeshClient struct {
	id string

	serviceCache *ServiceCache
	connection   *grpc.ClientConn
	grpcClient   mesh.MeshCtrlClient
	connected    bool
}

func NewMeshClient() *MeshClient {
	return &MeshClient{
		serviceCache: NewServiceCache(100),
	}
}

func (c *MeshClient) Connect(target string) {
	grpcConn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		klog.Error("Failed to create gRPC client: ", err)
		panic(err)
	}
	c.connection = grpcConn
	c.grpcClient = mesh.NewMeshCtrlClient(grpcConn)
	c.connected = true

}

func (c *MeshClient) Unsubscribe(ctx context.Context, serviceName string) error {
	return nil
}

func (c *MeshClient) Subscribe(ctx context.Context, serviceName string) error {
	subReq := &mesh.SubscriptionRequest{
		SidecarId:   c.id,
		ServiceName: serviceName,
	}
	stream, err := c.grpcClient.Subscribe(ctx, subReq)
	if err != nil {
		klog.Error("Failed to subscribe to events: ", err)
		return err
	}
	go func() {
		klog.Info("Subscribed to events successfully, waiting for events...")
		for {
			event, err := stream.Recv()
			if err != nil {
				klog.Error("Failed to receive event: ", err)
				return
			}
			klog.Infof("Received event: %+v\n", event)
			switch event.GetOpType() {
			case mesh.OpType_ADDED:
				klog.Info("Receive a service-add event\n")
				c.serviceCache.onAdd(serviceName, (*Endpoints)(event))
			case mesh.OpType_MODIFIED:
				klog.Info("Receive a service-modify event\n")
				c.serviceCache.onUpdate(serviceName, (*Endpoints)(event))
			case mesh.OpType_DELETED:
				klog.Info("Receive a service-delete event\n")
				c.serviceCache.onDelete(serviceName)
			}
		}
	}()
	return nil
}

func (c *MeshClient) GetServiceIps(serviceName string) map[string][]string {
	eps := c.serviceCache.GetEndpoints(serviceName)
	if eps == nil {
		return nil
	}
	return eps.GetIps()
}

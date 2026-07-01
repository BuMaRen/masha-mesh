package rpcclient

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
		klog.Errorf("[RpcClient] failed to create gRPC client: %v", err)
		panic(err)
	}
	return mesh.NewMeshCtrlClient(c)
}

type MeshClient struct {
	id           string
	remote       string
	serviceCache *ServiceCache
	connection   *grpc.ClientConn
	grpcClient   mesh.MeshCtrlClient
	connected    bool
}

func NewMeshClient(serviceCache *ServiceCache, opts *Options) *MeshClient {
	return &MeshClient{
		id:           opts.uid,
		serviceCache: serviceCache,
		remote:       opts.remote,
	}
}

func (c *MeshClient) Connect() {
	grpcConn, err := grpc.NewClient(c.remote, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		klog.Errorf("[RpcClient] failed to create gRPC client: %v", err)
		panic(err)
	}
	c.connection = grpcConn
	c.grpcClient = mesh.NewMeshCtrlClient(grpcConn)
	c.connected = true

}

func (c *MeshClient) Unsubscribe(_ context.Context, serviceName string) error {
	c.serviceCache.onDelete(serviceName)
	return nil
}

func (c *MeshClient) Subscribe(ctx context.Context, serviceName string) error {
	subReq := &mesh.SubscriptionRequest{
		SidecarId:   c.id,
		ServiceName: serviceName,
	}
	stream, err := c.grpcClient.Subscribe(ctx, subReq)
	if err != nil {
		klog.Errorf("[RpcClient] failed to subscribe to events: %v", err)
		return err
	}
	go func() {
		klog.Infof("[RpcClient] subscribed to service %s, waiting for events...", serviceName)
		for {
			event, err := stream.Recv()
			if err != nil {
				klog.Errorf("[RpcClient] failed to receive event for service %s: %v", serviceName, err)
				return
			}
			klog.V(4).Infof("[RpcClient] received event for service %s: %+v", serviceName, event)
			switch event.GetOpType() {
			case mesh.OpType_ADDED:
				klog.Infof("[RpcClient] [%s] service-add event", serviceName)
				c.serviceCache.onAdd(serviceName, (*Endpoints)(event))
			case mesh.OpType_MODIFIED:
				klog.Infof("[RpcClient] [%s] service-modify event", serviceName)
				c.serviceCache.onUpdate(serviceName, (*Endpoints)(event))
			case mesh.OpType_DELETED:
				klog.Infof("[RpcClient] [%s] service-delete event", serviceName)
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

package cli

import (
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

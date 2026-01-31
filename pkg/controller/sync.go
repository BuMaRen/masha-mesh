package controller

import (
	"context"

	"github.com/BuMaRen/mesh/pkg/api/mesh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Syncer struct {
	mesh.UnimplementedControlFaceServer
}

func (s *Syncer) Subscribe(*mesh.SubscribeRequest, grpc.ServerStreamingServer[mesh.ServiceUpdate]) error {
	return status.Error(codes.Unimplemented, "method Subscribe not implemented")
}
func (s *Syncer) Unsubsribe(context.Context, *mesh.SubscribeRequest) (*mesh.ServiceUpdate, error) {
	return nil, status.Error(codes.Unimplemented, "method Unsubsribe not implemented")
}
func (s *Syncer) ListService(context.Context, *mesh.SubscribeRequest) (*mesh.ServiceUpdate, error) {
	return nil, status.Error(codes.Unimplemented, "method ListService not implemented")
}

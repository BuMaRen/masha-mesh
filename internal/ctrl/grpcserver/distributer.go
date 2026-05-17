package grpcserver

import (
	"github.com/BuMaRen/mesh/internal/ctrl/distributer"
	"github.com/BuMaRen/mesh/pkg/api/mesh"
)

func (d *GrpcServer) Publish(svcName string, opType mesh.OpType, obj any) {
	sidecar := d.GetSidecar(svcName)
	if sidecar != nil {
		sidecar.Informer(opType, obj)
	}
}

func (d *GrpcServer) Distributer() distributer.Distributer {
	return d
}

var _ distributer.Distributer = (*GrpcServer)(nil)

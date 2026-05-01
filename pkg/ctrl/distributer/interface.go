package distributer

import "github.com/BuMaRen/mesh/pkg/api/mesh"

type Distributer interface {
	Publish(string, mesh.OpType, any)
}

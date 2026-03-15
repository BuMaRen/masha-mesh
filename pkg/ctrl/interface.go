package ctrl

import "github.com/BuMaRen/mesh/pkg/api/mesh"

type Distributer interface {
	Publish(string, mesh.OpType, any)
}

type Storage interface {
	OnAdded(obj any)
	OnUpdate(oldObj, newObj any)
	OnDeleted(obj any)
}

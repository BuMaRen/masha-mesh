package ctrl

type Distributer interface {
	Publish(string, any)
}

type Storage interface {
	OnAdded(obj any)
	OnUpdate(oldObj, newObj any)
	OnDeleted(obj any)
}

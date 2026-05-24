package reconciler

type Reconciler interface {
	OnAdded(any)
	OnUpdated(any, any)
	OnDeleted(any)
}

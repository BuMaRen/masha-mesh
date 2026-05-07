package data

type Cache interface {
	OnAdded(any) (bool, string)
	OnUpdate(any, any) (bool, string)
	OnDelete(any) (bool, string, bool)
	GetCache(string) (any, bool)
}

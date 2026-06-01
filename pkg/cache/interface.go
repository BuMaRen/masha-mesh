package cache

type Cache interface {
	OnAdded(any) (bool, string)
	OnUpdate(any, any) (bool, string)
	OnDelete(any) (bool, string, bool)
	GetCache(string) (any, bool)
}

type TypedCache[T any] interface {
	OnAdded(T) (bool, string)
	OnUpdate(T, T) (bool, string)
	OnDelete(T) (bool, string, bool)
	GetCache(string) (T, bool)
}

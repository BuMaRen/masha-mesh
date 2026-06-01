package resources

import (
	"sync"

	"github.com/BuMaRen/mesh/pkg/cache"
)

type ContainersCache struct {
	mtx  sync.Mutex
	ctns map[string]*Container
}

func NewContainersCache() *ContainersCache {
	return &ContainersCache{
		ctns: make(map[string]*Container),
		mtx:  sync.Mutex{},
	}
}

func (c *ContainersCache) OnAdded(obj any) (bool, string) {
	container := ParseContainer(obj)
	if container == nil {
		return false, ""
	}
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.ctns[container.Name] = container
	return true, container.Name
}

func (c *ContainersCache) OnUpdate(oldObj, newObj any) (bool, string) {
	container := ParseContainer(newObj)
	if container == nil {
		return false, ""
	}
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.ctns[container.Name] = container
	return true, container.Name
}

func (c *ContainersCache) OnDelete(obj any) (bool, string, bool) {
	container := ParseContainer(obj)
	if container == nil {
		return false, "", true
	}
	c.mtx.Lock()
	defer c.mtx.Unlock()
	delete(c.ctns, container.Name)
	return true, container.Name, true
}

func (c *ContainersCache) GetCache(name string) (any, bool) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	container, exist := c.ctns[name]
	if !exist {
		return nil, false
	}
	return container, true
}

var _ cache.Cache = (*ContainersCache)(nil)

package data

import (
	"github.com/BuMaRen/mesh/pkg/ctrl/resources"
)

type ContainersCache map[string]*resources.Container

func NewContainersCache() ContainersCache {
	return make(map[string]*resources.Container)
}

func (c ContainersCache) OnAdded(obj any) (bool, string) {
	container := resources.ParseContainer(obj)
	if container == nil {
		return false, ""
	}
	c[container.Name] = container
	return true, container.Name
}

func (c ContainersCache) OnUpdate(oldObj, newObj any) (bool, string) {
	container := resources.ParseContainer(newObj)
	if container == nil {
		return false, ""
	}
	c[container.Name] = container
	return true, container.Name
}

func (c ContainersCache) OnDelete(obj any) (bool, string, bool) {
	container := resources.ParseContainer(obj)
	if container == nil {
		return false, "", false
	}
	delete(c, container.Name)
	return true, container.Name, true
}

func (c ContainersCache) GetCache(name string) (any, bool) {
	container, exist := c[name]
	if !exist {
		return nil, false
	}
	return container, true
}

type ContainerQueueItem struct {
	Key string
	Op  string
}

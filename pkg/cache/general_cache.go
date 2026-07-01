package cache

import "sync"

type GeneralCache[T any] struct {
	mtx  *sync.RWMutex
	data map[string]T
}

func (c *GeneralCache[T]) Add(key string, value T) (T, bool) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if old, exist := c.data[key]; exist {
		// 已存在，返回旧值和 false
		return old, false
	}
	c.data[key] = value
	return value, true
}

func (c *GeneralCache[T]) Update(key string, value T) (T, bool) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if old, exist := c.data[key]; !exist {
		// 不存在，无法更新，返回零值和 false
		return old, false
	}
	c.data[key] = value
	return value, true
}

func (c *GeneralCache[T]) Delete(key string) (T, bool) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if old, exist := c.data[key]; !exist {
		// 不存在，无法删除，返回零值和 false
		return old, false
	} else {
		delete(c.data, key)
		return old, true
	}
}

func (c *GeneralCache[T]) Get(key string) (T, bool) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	value, exist := c.data[key]
	return value, exist
}

func NewGeneralCache[T any]() *GeneralCache[T] {
	return &GeneralCache[T]{
		mtx:  &sync.RWMutex{},
		data: make(map[string]T),
	}
}

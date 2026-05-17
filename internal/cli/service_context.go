package cli

// 管理订阅Service所用的context
// 维护一个map，key是serviceName，value是一个cancelFunc
// 当取消订阅时调用这个cancelFunc来取消对应的context

import (
	"context"
	"sync"
)

type ServiceContext struct {
	mtx    *sync.RWMutex
	ctxMap map[string]context.CancelFunc
}

func NewServiceContext() *ServiceContext {
	return &ServiceContext{
		mtx:    &sync.RWMutex{},
		ctxMap: make(map[string]context.CancelFunc),
	}
}

func (s *ServiceContext) NewServiceContext(parent context.Context, serviceName string) context.Context {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	ctx, cancel := context.WithCancel(parent)
	s.ctxMap[serviceName] = cancel
	return ctx
}

func (s *ServiceContext) CancelServiceContext(serviceName string) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if cancel, exists := s.ctxMap[serviceName]; exists {
		cancel()
		delete(s.ctxMap, serviceName)
	}
}

func (s *ServiceContext) GetCancel(serviceName string) (context.CancelFunc, bool) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	cancel, exists := s.ctxMap[serviceName]
	if !exists {
		return nil, false
	}
	return cancel, true
}

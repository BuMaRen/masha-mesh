package cli

import (
	"sync"

	"github.com/BuMaRen/mesh/pkg/api/mesh"
)

type Endpoints mesh.ClientSubscriptionEvent

func NewEndpoints(event *mesh.ClientSubscriptionEvent) *Endpoints {
	return (*Endpoints)(event)
}

func (e *Endpoints) GetIps() map[string][]string {
	epsMap := make(map[string][]string, len(e.Endpoints))
	for epsName, ips := range e.Endpoints {
		epsMap[epsName] = ips.GetEndpointIps()
	}
	return epsMap
}

func (e *Endpoints) GetIpsByName(epsName string) []string {
	eps, existed := e.Endpoints[epsName]
	if !existed {
		return []string{}
	}
	return eps.GetEndpointIps()
}

type ServiceCache struct {
	mtx   *sync.RWMutex
	cache map[string]*Endpoints
}

func NewServiceCache(capacity int) *ServiceCache {
	return &ServiceCache{
		mtx:   &sync.RWMutex{},
		cache: make(map[string]*Endpoints, capacity),
	}
}

// onDelete 整个 service 的删除
func (s *ServiceCache) onDelete(serviceName string) {
	s.mtx.Lock()
	delete(s.cache, serviceName)
	s.mtx.Unlock()
}

// onUpdate 整个 service 的更新
func (s *ServiceCache) onUpdate(serviceName string, eps ...*Endpoints) {
	s.mtx.Lock()
	// 对比还是直接覆盖？目前先直接覆盖，后续如果有需要再对比
	if len(eps) > 0 {
		s.cache[serviceName] = eps[len(eps)-1]
	}
	s.mtx.Unlock()
}

// onAdd 整个 service 的添加
func (s *ServiceCache) onAdd(serviceName string, eps *Endpoints) {
	s.mtx.Lock()
	// 直接覆盖，如果之前存在的话
	s.cache[serviceName] = eps
	s.mtx.Unlock()
}

// GetEndpoints 获取 service 对应的 eps 结构体
func (s *ServiceCache) GetEndpoints(serviceName string) *Endpoints {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	eps, existed := s.cache[serviceName]
	if !existed {
		return nil
	}
	return eps
}

// GetServiceIps 获取 service 对应的 eps map
func (s *ServiceCache) GetServiceIps(serviceName string) map[string][]string {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	eps, existed := s.cache[serviceName]
	if !existed {
		return map[string][]string{}
	}
	return eps.GetIps()
}

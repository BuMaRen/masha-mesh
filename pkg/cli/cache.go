package cli

import "github.com/BuMaRen/mesh/pkg/api/mesh"

type Endpoints mesh.ClientSubscriptionEvent

func NewEndpoints(event *mesh.ClientSubscriptionEvent) *Endpoints {
	return (*Endpoints)(event)
}

func (e *Endpoints) GetIpsByName(name string) []string {
	eps, existed := e.Endpoints[name]
	if !existed {
		return []string{}
	}
	return eps.GetEndpointIps()
}

type ServiceCache map[string]*Endpoints

func NewServiceCache(capacity int) ServiceCache {
	return make(ServiceCache, capacity)
}

func (s ServiceCache) onDelete(name string) {
	delete(s, name)
}

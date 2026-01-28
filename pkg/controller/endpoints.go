package controller

type EndpointSlices struct {
	ServiceName  string
	EndpointList Endpoints
}

func (e *EndpointSlices) RemoveInstance(instanceId string) {
	e.EndpointList.removeInstance(instanceId)
}


type Endpoints map[string][]string

func newEndpoint() Endpoints {
	return make(Endpoints)
}

func (e Endpoints) removeInstance(instanceId string) {
	if _, exists := e[instanceId]; exists {
		delete(e, instanceId)
	}
}

func (e Endpoints) replaceInstance(instanceId string, addresses []string) {
	e[instanceId] = addresses
}

func (e Endpoints) removeAddresses(instanceId string, addresses []string) {
	mp := make(map[string]struct{})
	for _, addr := range addresses {
		mp[addr] = struct{}{}
	}
	var filtered []string
	for _, addr := range e[instanceId] {
		if _, exists := mp[addr]; !exists {
			filtered = append(filtered, addr)
		}
	}
	e[instanceId] = filtered
	if len(e[instanceId]) == 0 {
		delete(e, instanceId)
	}
}

func (e Endpoints) appendAddresses(instanceId string, addresses []string) {
	if _, exists := e[instanceId]; !exists {
		e[instanceId] = []string{}
	}
	e[instanceId] = append(e[instanceId], addresses...)
}

func (e Endpoints) mergeAddresses(instanceId string, addresses []string) {
	if _, exists := e[instanceId]; !exists {
		e[instanceId] = []string{}
	}
	mp := make(map[string]struct{})
	for _, addr := range e[instanceId] {
		mp[addr] = struct{}{}
	}
	for _, addr := range addresses {
		mp[addr] = struct{}{}
	}
	var merged []string
	for addr := range mp {
		merged = append(merged, addr)
	}
	e[instanceId] = merged
}

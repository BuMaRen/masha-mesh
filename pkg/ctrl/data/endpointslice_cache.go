package data

import (
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// D_EndpointSlice 存放单个 Service 的 EndpointSlice
type D_EndpointSlice struct {
	serviceName  string
	epsNameToEps map[string]*discoveryv1.EndpointSlice
}

func NewD_EndpointSlice(serviceName string) *D_EndpointSlice {
	return &D_EndpointSlice{
		serviceName:  serviceName,
		epsNameToEps: make(map[string]*discoveryv1.EndpointSlice),
	}
}

func (d *D_EndpointSlice) ServiceName() string {
	return d.serviceName
}

func (d *D_EndpointSlice) GetMerged() *discoveryv1.EndpointSlice {
	merged := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:   d.serviceName,
			Labels: map[string]string{"kubernetes.io/service-name": d.serviceName},
		},
	}
	for _, es := range d.epsNameToEps {
		merged.Endpoints = append(merged.Endpoints, es.Endpoints...)
	}
	return merged
}

func (d *D_EndpointSlice) OnAdded(es *discoveryv1.EndpointSlice) {
	if es.Labels["kubernetes.io/service-name"] != d.serviceName {
		return
	}
	d.epsNameToEps[es.Name] = es.DeepCopy()
}

func (d *D_EndpointSlice) OnDelete(es *discoveryv1.EndpointSlice) {
	if es.Labels["kubernetes.io/service-name"] != d.serviceName {
		return
	}
	delete(d.epsNameToEps, es.Name)
}

func (d *D_EndpointSlice) OnUpdate(oldEs, newEs *discoveryv1.EndpointSlice) {
	if oldEs.Labels["kubernetes.io/service-name"] != d.serviceName {
		return
	}
	if newEs.Labels["kubernetes.io/service-name"] != d.serviceName {
		return
	}
	delete(d.epsNameToEps, oldEs.Name)
	d.epsNameToEps[newEs.Name] = newEs.DeepCopy()
}

// EndpointSliceCache 存放所有 Service 的 EndpointSlice
type EndpointSliceCache map[string]*D_EndpointSlice

func NewEndpointSliceCache() EndpointSliceCache {
	return make(map[string]*D_EndpointSlice)
}

func (c EndpointSliceCache) OnAdded(obj any) (bool, string) {
	es, ok := obj.(*discoveryv1.EndpointSlice)
	if !ok {
		return false, ""
	}
	svcName := es.Labels["kubernetes.io/service-name"]
	if _, exist := c[svcName]; !exist {
		c[svcName] = NewD_EndpointSlice(svcName)
	}
	c[svcName].OnAdded(es)
	return true, svcName
}

func (c EndpointSliceCache) OnUpdate(oldObj, newObj any) (bool, string) {
	oldEs, ok1 := oldObj.(*discoveryv1.EndpointSlice)
	newEs, ok2 := newObj.(*discoveryv1.EndpointSlice)
	if !ok1 || !ok2 {
		return false, ""
	}

	svcName := oldEs.Labels["kubernetes.io/service-name"]
	if _, exist := c[svcName]; !exist {
		c[svcName] = NewD_EndpointSlice(svcName)
	}
	c[svcName].OnUpdate(oldEs, newEs)
	return true, svcName
}

func (c EndpointSliceCache) OnDelete(obj any) (bool, string, bool) {
	es, ok := obj.(*discoveryv1.EndpointSlice)
	if !ok {
		return false, "", false
	}
	svcName := es.Labels["kubernetes.io/service-name"]
	if _, exist := c[svcName]; !exist {
		return false, "", false
	}
	c[svcName].OnDelete(es)
	if len(c[svcName].epsNameToEps) == 0 {
		delete(c, svcName)
	}
	return true, svcName, len(c[svcName].epsNameToEps) == 0
}

func (c EndpointSliceCache) GetMerged(svcName string) (any, bool) {
	dEs, exist := c[svcName]
	if !exist {
		return nil, false
	}
	return dEs.GetMerged(), true
}

var _ Cache = EndpointSliceCache{}

package ctrl

import (
	"github.com/BuMaRen/mesh/pkg/ctrl/utils"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// EndpointSlice 用于存放一个 service 对应的多个 EndpointSlice
type EndpointSlice struct {
	serviceName string
	// key 为 EndpointSlice 的名字，value 为 EndpointSlice 对象
	esNameToEs map[string]*discoveryv1.EndpointSlice
}

func versionMatched(oldEs, newEs *discoveryv1.EndpointSlice) bool {
	if !utils.VersionIncrement(oldEs.ObjectMeta.ResourceVersion, newEs.ObjectMeta.ResourceVersion) {
		// TODO: 版本不对，可能是丢失了某次更新，或者更新顺序错了，需要重新拉取最新的 EndpointSlice 列表
		klog.Warningf("EndpointSlice %s version not incremented correctly: current=%s, incoming=%s", newEs.Name, oldEs.ObjectMeta.ResourceVersion, newEs.ObjectMeta.ResourceVersion)
		return false
	}
	return true
}

func (e *EndpointSlice) OnAdded(endpointSlice *discoveryv1.EndpointSlice) {
	// 二次检查，如果不是同一个 Service 就不操作
	svcName := endpointSlice.Labels["kubernetes.io/service-name"]
	if e.serviceName == "" || e.serviceName != svcName {
		return
	}
	// 同一个 Service，根据 EndpointSlice 区分处理
	curEs, exist := e.esNameToEs[endpointSlice.Name]
	if !exist || versionMatched(curEs, endpointSlice) {
		e.esNameToEs[endpointSlice.Name] = endpointSlice.DeepCopy()
	}
}

func (e *EndpointSlice) OnUpdate(oldOne, newOne *discoveryv1.EndpointSlice) {
	svcName := oldOne.Labels["kubernetes.io/service-name"]
	if e.serviceName == "" || e.serviceName != svcName {
		return
	}
	if _, exist := e.esNameToEs[oldOne.Name]; !exist {
		klog.Warningf("updating a endpointSlice for service %s. for non-existed endpointSlice, do nothing", svcName)
		return
	}
	if versionMatched(oldOne, newOne) {
		delete(e.esNameToEs, oldOne.Name)
		e.esNameToEs[newOne.Name] = newOne.DeepCopy()
	}
}

func (e *EndpointSlice) OnDelete(endpointSlice *discoveryv1.EndpointSlice) {
	svcName := endpointSlice.Labels["kubernetes.io/service-name"]
	if e.serviceName == "" || e.serviceName != svcName {
		return
	}
	if _, exist := e.esNameToEs[endpointSlice.Name]; !exist {
		klog.Warningf("deleting a endpointSlice for service %s. for non-existed endpointSlice, do nothing", svcName)
		return
	}
	delete(e.esNameToEs, endpointSlice.Name)
}

// Merge 将一个 service 对应的多个 EndpointSlice 合并成一个 EndpointSlice，方便发送给 sidecar
func (e *EndpointSlice) Merge() *discoveryv1.EndpointSlice {
	merged := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:   e.serviceName,
			Labels: map[string]string{"kubernetes.io/service-name": e.serviceName},
		},
	}
	for _, es := range e.esNameToEs {
		merged.Endpoints = append(merged.Endpoints, es.Endpoints...)
	}
	return merged
}

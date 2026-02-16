package ctrl

import (
	"github.com/BuMaRen/mesh/pkg/api/mesh"
	"github.com/BuMaRen/mesh/pkg/ctrl/utils"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/klog/v2"
)

type CoreData struct {
	distributer Distributer
	// TODO：一个service可能对应多个endpointSlice，需要自己整合成一个数据结构，方便发送给sidecar
	endpointSliceMap map[string]*discoveryv1.EndpointSlice
}

func (d *CoreData) List(svcName string) (*discoveryv1.EndpointSlice, bool) {
	eps, exist := d.endpointSliceMap[svcName]
	return eps, exist
}

func (d *CoreData) OnAdded(obj any) {
	endpointSlice := obj.(*discoveryv1.EndpointSlice)
	svcName := endpointSlice.Labels["kubernetes.io/service-name"]
	endpointName := endpointSlice.Name
	curEps, exists := d.endpointSliceMap[svcName]
	if !exists {
		// New EndpointSlice
		d.endpointSliceMap[svcName] = endpointSlice.DeepCopy()
		// Publish 将内容写给 sidecar 的 channel，grpc 的 stream 再将从 channel 里读到的内容发给 sidecar
		d.distributer.Publish(svcName, mesh.OpType_ADDED, d.endpointSliceMap[svcName])
		klog.Infof("Added new EndpointSlice %s for service %s", endpointName, svcName)
		return
	}
	rVersion := curEps.ObjectMeta.ResourceVersion
	if !utils.VersionIncrement(rVersion, endpointSlice.ObjectMeta.ResourceVersion) {
		klog.Warningf("EndpointSlice %s version not incremented correctly: current=%s, incoming=%s", svcName, rVersion, endpointSlice.ObjectMeta.ResourceVersion)
		// TODO: handle version mismatch
		return
	}
	// TODO: 处理多个 endpointSlice 的合并逻辑，目前先简单地覆盖掉
	d.endpointSliceMap[svcName] = endpointSlice.DeepCopy()
	d.distributer.Publish(svcName, mesh.OpType_ADDED, d.endpointSliceMap[svcName])
	klog.Infof("Added existing EndpointSlice %s for service %s with updated version", endpointName, svcName)
}

func (d *CoreData) OnUpdate(oldObj, newObj any) {
	endpointSlice := newObj.(*discoveryv1.EndpointSlice)
	svcName := endpointSlice.Labels["kubernetes.io/service-name"]
	endpointName := endpointSlice.Name
	curEps, exists := d.endpointSliceMap[svcName]
	if !exists {
		// Treat as new addition
		d.endpointSliceMap[svcName] = endpointSlice.DeepCopy()
		d.distributer.Publish(svcName, mesh.OpType_MODIFIED, d.endpointSliceMap[svcName])
		klog.Infof("Updated EndpointSlice %s for service %s which did not exist before, treated as addition", endpointName, svcName)
		return
	}
	rVersion := curEps.ObjectMeta.ResourceVersion
	if !utils.VersionIncrement(rVersion, endpointSlice.ObjectMeta.ResourceVersion) {
		klog.Warningf("EndpointSlice %s version not incremented correctly: current=%s, incoming=%s", svcName, rVersion, endpointSlice.ObjectMeta.ResourceVersion)
		// TODO: handle version mismatch
		return
	}
	// TODO: 处理多个 endpointSlice 的合并逻辑，目前先简单地覆盖掉
	d.endpointSliceMap[svcName] = endpointSlice.DeepCopy()
	d.distributer.Publish(svcName, mesh.OpType_MODIFIED, d.endpointSliceMap[svcName])
	klog.Infof("Updated EndpointSlice %s for service %s with new version", endpointName, svcName)
}

func (d *CoreData) OnDeleted(obj any) {
	endpointSlice := obj.(*discoveryv1.EndpointSlice)
	svcName := endpointSlice.Labels["kubernetes.io/service-name"]
	endpointName := endpointSlice.Name
	if _, exist := d.endpointSliceMap[svcName]; !exist {
		klog.Warningf("Attempted to delete non-existent EndpointSlice %s for service %s", endpointName, svcName)
		return
	}
	delete(d.endpointSliceMap, svcName)
	endpointSlice.Endpoints = []discoveryv1.Endpoint{}
	d.distributer.Publish(svcName, mesh.OpType_DELETED, endpointSlice)
	klog.Infof("Deleted EndpointSlice %s for service %s", endpointName, svcName)
}

func NewCoreData(distributer Distributer) *CoreData {
	return &CoreData{
		distributer:      distributer,
		endpointSliceMap: make(map[string]*discoveryv1.EndpointSlice),
	}
}

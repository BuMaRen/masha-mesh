package ctrl

import (
	"github.com/BuMaRen/mesh/pkg/ctrl/utils"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/klog/v2"
)

type CoreData struct {
	distributer Distributer
	// TODO：一个service可能对应多个endpointSlice
	// TODO：多个endpointSlice的ResourceVersion不一定相同，CoreData需要自己维护一个，用于和sidecar同步
	endpointSliceMap map[string]map[string]*discoveryv1.EndpointSlice
}

func (d *CoreData) OnAdded(obj any) {
	endpointSlice := obj.(*discoveryv1.EndpointSlice)
	svcName := endpointSlice.Labels["kubernetes.io/service-name"]
	endpointName := endpointSlice.Name
	curEps, exists := d.endpointSliceMap[svcName]
	if !exists {
		// New EndpointSlice
		d.endpointSliceMap[svcName] = make(map[string]*discoveryv1.EndpointSlice)
		d.endpointSliceMap[svcName][endpointName] = endpointSlice.DeepCopy()
		// Publish 将内容写给 sidecar 的 channel，grpc 的 stream 再将从 channel 里读到的内容发给 sidecar
		d.distributer.Publish(svcName, d.endpointSliceMap[svcName])
		klog.Infof("Added new EndpointSlice %s for service %s", endpointName, svcName)
		return
	}
	rVersion := curEps[endpointName].ObjectMeta.ResourceVersion
	if !utils.VersionIncrement(rVersion, endpointSlice.ObjectMeta.ResourceVersion) {
		klog.Warningf("EndpointSlice %s version not incremented correctly: current=%s, incoming=%s", svcName, rVersion, endpointSlice.ObjectMeta.ResourceVersion)
		// TODO: handle version mismatch
		return
	}
	d.endpointSliceMap[svcName][endpointName] = endpointSlice.DeepCopy()
	d.distributer.Publish(svcName, d.endpointSliceMap[svcName])
	klog.Infof("Added existing EndpointSlice %s for service %s with updated version", endpointName, svcName)
}

func (d *CoreData) OnUpdate(oldObj, newObj any) {
	endpointSlice := newObj.(*discoveryv1.EndpointSlice)
	svcName := endpointSlice.Labels["kubernetes.io/service-name"]
	endpointName := endpointSlice.Name
	curEps, exists := d.endpointSliceMap[svcName]
	if !exists {
		// Treat as new addition
		d.endpointSliceMap[svcName] = make(map[string]*discoveryv1.EndpointSlice)
		d.endpointSliceMap[svcName][endpointName] = endpointSlice.DeepCopy()
		d.distributer.Publish(svcName, d.endpointSliceMap[svcName])
		klog.Infof("Updated EndpointSlice %s for service %s which did not exist before, treated as addition", endpointName, svcName)
		return
	}
	rVersion := curEps[endpointName].ObjectMeta.ResourceVersion
	if !utils.VersionIncrement(rVersion, endpointSlice.ObjectMeta.ResourceVersion) {
		klog.Warningf("EndpointSlice %s version not incremented correctly: current=%s, incoming=%s", svcName, rVersion, endpointSlice.ObjectMeta.ResourceVersion)
		// TODO: handle version mismatch
		return
	}
	d.endpointSliceMap[svcName][endpointName] = endpointSlice.DeepCopy()
	d.distributer.Publish(svcName, d.endpointSliceMap[svcName])
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
	delete(d.endpointSliceMap[endpointName], svcName)
	d.distributer.Publish(svcName, nil)
	klog.Infof("Deleted EndpointSlice %s for service %s", endpointName, svcName)
}

func NewCoreData(distributer Distributer) *CoreData {
	return &CoreData{
		distributer:      distributer,
		endpointSliceMap: make(map[string]map[string]*discoveryv1.EndpointSlice),
	}
}

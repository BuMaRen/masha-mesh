package ctrl

import (
	"github.com/BuMaRen/mesh/pkg/api/mesh"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/klog/v2"
)

type CoreData struct {
	distributer Distributer
	// serviceMap 存放一个 service 对应的多个 EndpointSlice，key 是 service 的名字，value 是 EndpointSlice 对象
	serviceMap map[string]*EndpointSlice
}

func (d *CoreData) List(svcName string) (*discoveryv1.EndpointSlice, bool) {
	es, existed := d.serviceMap[svcName]
	if !existed {
		klog.Warningf("endpointSlice cache of service %s not found", svcName)
		return nil, false
	}
	return es.Merge(), true
}

func (d *CoreData) OnAdded(obj any) {
	endpointSlice := obj.(*discoveryv1.EndpointSlice)
	svcName := endpointSlice.Labels["kubernetes.io/service-name"]
	if _, exist := d.serviceMap[svcName]; !exist {
		d.serviceMap[svcName] = &EndpointSlice{
			serviceName: svcName,
			esNameToEs:  make(map[string]*discoveryv1.EndpointSlice),
		}
	}
	// resourceVersion 的检查放在 EndpointSlice 里做
	d.serviceMap[svcName].OnAdded(endpointSlice)

	serviceEs := d.serviceMap[svcName].Merge()
	// TODO: 目前sidecar只能对service粒度做订阅，endpointSlice的新增会需要sidecar更新整个Service的缓存
	d.distributer.Publish(svcName, mesh.OpType_ADDED, serviceEs)
	klog.Infof("Added EndpointSlice for service %s with version %s", svcName, serviceEs.ObjectMeta.ResourceVersion)
}

func (d *CoreData) OnUpdate(oldObj, newObj any) {
	endpointSlice := newObj.(*discoveryv1.EndpointSlice)
	svcName := endpointSlice.Labels["kubernetes.io/service-name"]
	if _, exist := d.serviceMap[svcName]; !exist {
		d.serviceMap[svcName] = &EndpointSlice{
			serviceName: svcName,
			esNameToEs:  make(map[string]*discoveryv1.EndpointSlice),
		}
	}
	// resourceVersion 的检查放在 EndpointSlice 里做
	d.serviceMap[svcName].OnUpdate(oldObj.(*discoveryv1.EndpointSlice), newObj.(*discoveryv1.EndpointSlice))

	serviceEs := d.serviceMap[svcName].Merge()
	// TODO: 目前sidecar只能对service粒度做订阅，endpointSlice的更新会需要sidecar更新整个Service的缓存（https://github.com/users/BuMaRen/projects/4/views/1?pane=issue&itemId=158944635&issue=BuMaRen%7Cmasha-mesh%7C22）
	d.distributer.Publish(svcName, mesh.OpType_MODIFIED, serviceEs)
	klog.Infof("Updated EndpointSlice for service %s with version %s", svcName, serviceEs.ObjectMeta.ResourceVersion)
}

func (d *CoreData) OnDeleted(obj any) {
	endpointSlice := obj.(*discoveryv1.EndpointSlice)
	svcName := endpointSlice.Labels["kubernetes.io/service-name"]
	if _, exist := d.serviceMap[svcName]; !exist {
		klog.Warningf("Attempted to delete EndpointSlice for non-existent service %s", svcName)
		return
	}
	d.serviceMap[svcName].OnDelete(endpointSlice)

	serviceEs := d.serviceMap[svcName].Merge()
	// TODO: 目前sidecar只能对service粒度做订阅，endpointSlice的删除(service的其他endpointSlice还在)对于sidecar来说只是一次更新
	if len(serviceEs.Endpoints) == 0 {
		d.distributer.Publish(svcName, mesh.OpType_DELETED, serviceEs)
		delete(d.serviceMap, svcName)
		klog.Infof("Deleted all EndpointSlices for service %s with version %s", svcName, serviceEs.ObjectMeta.ResourceVersion)
		return
	}
	// service 还有其他 EndpointSlice，发布更新事件
	d.distributer.Publish(svcName, mesh.OpType_MODIFIED, serviceEs)
	klog.Infof("Deleted EndpointSlice for service %s with version %s", svcName, serviceEs.ObjectMeta.ResourceVersion)
}

func NewCoreData(distributer Distributer) *CoreData {
	return &CoreData{
		distributer: distributer,
		serviceMap:  make(map[string]*EndpointSlice),
	}
}

package k8scontroller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type CrdController struct {
	// 存下 crd 方便于 webhook 注入
	resourceCache map[string]any
}

func NewController() *CrdController {
	return &CrdController{
		resourceCache: make(map[string]any),
	}
}

func (c *CrdController) OnAdded(obj any) {
	metaObj, ok := obj.(*metav1.ObjectMeta)
	if !ok {
		klog.Warningf("Received object is not of type *metav1.ObjectMeta: %v", obj)
		return
	}
	// TODO: index of CRD
	crdName := metaObj.Name
	c.resourceCache[crdName] = obj
}

func (c *CrdController) OnUpdated(oldObj, newObj any) {
	metaObj, ok := newObj.(*metav1.ObjectMeta)
	if !ok {
		klog.Warningf("Received object is not of type *metav1.ObjectMeta: %v", newObj)
		return
	}
	crdName := metaObj.Name
	c.resourceCache[crdName] = newObj

	// TODO: update resource that depends on this CRD
}

func (c *CrdController) OnDeleted(obj any) {
	metaObj, ok := obj.(*metav1.ObjectMeta)
	if !ok {
		klog.Warningf("Received object is not of type *metav1.ObjectMeta: %v", obj)
		return
	}
	crdName := metaObj.Name
	delete(c.resourceCache, crdName)

	// TODO: update resource that depends on this CRD

}

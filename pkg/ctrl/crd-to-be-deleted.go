package ctrl

import (
	"context"
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// CRDEventHandler defines callbacks for CRD watch events.
type CRDEventHandler interface {
	OnAdd(obj *unstructured.Unstructured)
	OnUpdate(oldObj, newObj *unstructured.Unstructured)
	OnDelete(obj *unstructured.Unstructured)
}

// CRDEventHandlerFuncs is a helper to create handlers from plain functions.
type CRDEventHandlerFuncs struct {
	AddFunc    func(obj *unstructured.Unstructured)
	UpdateFunc func(oldObj, newObj *unstructured.Unstructured)
	DeleteFunc func(obj *unstructured.Unstructured)
}

func (f CRDEventHandlerFuncs) OnAdd(obj *unstructured.Unstructured) {
	if f.AddFunc != nil {
		f.AddFunc(obj)
	}
}

func (f CRDEventHandlerFuncs) OnUpdate(oldObj, newObj *unstructured.Unstructured) {
	if f.UpdateFunc != nil {
		f.UpdateFunc(oldObj, newObj)
	}
}

func (f CRDEventHandlerFuncs) OnDelete(obj *unstructured.Unstructured) {
	if f.DeleteFunc != nil {
		f.DeleteFunc(obj)
	}
}

// CRDController watches a single GVR and dispatches event callbacks.
type CRDController struct {
	gvr          schema.GroupVersionResource
	namespace    string
	resyncPeriod time.Duration
	handler      CRDEventHandler
}

func NewCRDController(gvr schema.GroupVersionResource, namespace string, resyncPeriod time.Duration, handler CRDEventHandler) *CRDController {
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}
	if resyncPeriod < 0 {
		resyncPeriod = 0
	}

	return &CRDController{
		gvr:          gvr,
		namespace:    namespace,
		resyncPeriod: resyncPeriod,
		handler:      handler,
	}
}

func (c *CRDController) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is nil")
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("build in-cluster config failed: %w", err)
	}

	return c.RunWithConfig(ctx, config)
}

func (c *CRDController) RunWithConfig(ctx context.Context, config *rest.Config) error {
	if config == nil {
		return errors.New("kubernetes rest config is nil")
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("create dynamic client failed: %w", err)
	}

	return c.RunWithClient(ctx, dynamicClient)
}

func (c *CRDController) RunWithClient(ctx context.Context, dynamicClient dynamic.Interface) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	if dynamicClient == nil {
		return errors.New("dynamic client is nil")
	}
	if c.handler == nil {
		return errors.New("crd event handler is nil")
	}

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicClient, c.resyncPeriod, c.namespace, nil)
	informer := factory.ForResource(c.gvr).Informer()

	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			u, ok := obj.(*unstructured.Unstructured)
			if !ok {
				klog.Warningf("[CRDController] unexpected add object type: %T", obj)
				return
			}
			c.handler.OnAdd(u)
		},
		UpdateFunc: func(oldObj, newObj any) {
			oldU, okOld := oldObj.(*unstructured.Unstructured)
			newU, okNew := newObj.(*unstructured.Unstructured)
			if !okOld || !okNew {
				klog.Warningf("[CRDController] unexpected update object type old=%T new=%T", oldObj, newObj)
				return
			}
			if oldU.GetResourceVersion() == newU.GetResourceVersion() {
				return
			}
			c.handler.OnUpdate(oldU, newU)
		},
		DeleteFunc: func(obj any) {
			u, err := toUnstructured(obj)
			if err != nil {
				klog.Warningf("[CRDController] decode delete object failed: %v", err)
				return
			}
			c.handler.OnDelete(u)
		},
	}); err != nil {
		return fmt.Errorf("register informer handlers failed: %w", err)
	}

	klog.Infof("[CRDController] start watch gvr=%s namespace=%s", c.gvr.String(), c.namespace)
	factory.Start(ctx.Done())
	if ok := cache.WaitForCacheSync(ctx.Done(), informer.HasSynced); !ok {
		return fmt.Errorf("sync cache failed for gvr=%s", c.gvr.String())
	}

	<-ctx.Done()
	klog.Infof("[CRDController] stop watch gvr=%s namespace=%s", c.gvr.String(), c.namespace)
	return nil
}

func toUnstructured(obj any) (*unstructured.Unstructured, error) {
	switch t := obj.(type) {
	case *unstructured.Unstructured:
		return t, nil
	case cache.DeletedFinalStateUnknown:
		u, ok := t.Obj.(*unstructured.Unstructured)
		if !ok {
			return nil, fmt.Errorf("unexpected tombstone object type: %T", t.Obj)
		}
		return u, nil
	default:
		return nil, fmt.Errorf("unexpected object type: %T", obj)
	}
}

// LoggingCRDHandler is a simple handler useful for smoke tests and debugging.
type LoggingCRDHandler struct{}

func (LoggingCRDHandler) OnAdd(obj *unstructured.Unstructured) {
	klog.Infof("[CRDController] add %s/%s rv=%s", obj.GetNamespace(), obj.GetName(), obj.GetResourceVersion())
}

func (LoggingCRDHandler) OnUpdate(oldObj, newObj *unstructured.Unstructured) {
	klog.Infof("[CRDController] update %s/%s rv=%s->%s", newObj.GetNamespace(), newObj.GetName(), oldObj.GetResourceVersion(), newObj.GetResourceVersion())
}

func (LoggingCRDHandler) OnDelete(obj *unstructured.Unstructured) {
	klog.Infof("[CRDController] delete %s/%s rv=%s", obj.GetNamespace(), obj.GetName(), obj.GetResourceVersion())
}

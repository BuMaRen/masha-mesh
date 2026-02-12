package ctrl

import (
	"context"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

func (l *Logic) endpointSliceInformer(ctx context.Context) {
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}
	clientSet := kubernetes.NewForConfigOrDie(kubeConfig)

	informerFactory := informers.NewSharedInformerFactory(clientSet, 0)

	endpointSliceInformer := informerFactory.Discovery().V1().EndpointSlices()

	endpointSliceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) {},
		UpdateFunc: func(oldObj, newObj any) {},
		DeleteFunc: func(obj any) {},
	})

	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())
}

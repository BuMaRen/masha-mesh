package utils

import (
	"sync"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var inClusterConfig = sync.OnceValues(func() (*rest.Config, error) { return rest.InClusterConfig() })

func InClusterConfigOrDie() *rest.Config {
	cfg, err := inClusterConfig()
	if err != nil {
		panic(err)
	}
	return cfg
}

func NewDynamicClientOrDie() *dynamic.DynamicClient {
	cfg := InClusterConfigOrDie()
	return dynamic.NewForConfigOrDie(cfg)
}

func NewKubernetesClientOrDie() *kubernetes.Clientset {
	cfg := InClusterConfigOrDie()
	return kubernetes.NewForConfigOrDie(cfg)
}

package utils

import (
	"sync"

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

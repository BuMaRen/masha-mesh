package utils

import (
	"strconv"
	"sync"

	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

var inClusterConfig = sync.OnceValues(func() (*rest.Config, error) { return rest.InClusterConfig() })

func InClusterConfigOrDie() *rest.Config {
	cfg, err := inClusterConfig()
	if err != nil {
		panic(err)
	}
	return cfg
}

// VersionIncrement checks if the incomingVersion is exactly one increment higher than the currentVersion.
func VersionIncrement(currentVersion, incomingVersion string) bool {
	// 当前假设版本格式均为数字，单调递增
	curVer, err1 := strconv.Atoi(currentVersion)
	incVer, err2 := strconv.Atoi(incomingVersion)
	if err1 != nil || err2 != nil {
		klog.Error("Version format error, should be numeric string")
		return false
	}
	klog.Infof("Comparing versions: current=%d, incoming=%d", curVer, incVer)
	return incVer-curVer == 1
}

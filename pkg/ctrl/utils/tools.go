package utils

import (
	"strconv"

	"k8s.io/klog/v2"
)

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

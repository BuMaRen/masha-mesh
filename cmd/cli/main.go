package main

import (
	"os"

	"k8s.io/klog/v2"
)

func main() {
	command := NewCommand()
	if err := command.Execute(); err != nil {
		klog.Errorf("command execution failed with error: %+v", err)
		os.Exit(1)
	}
}

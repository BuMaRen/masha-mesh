package main

import (
	"os"

	"github.com/BuMaRen/mesh/cmd/cli/app"
	"k8s.io/klog/v2"
)

func main() {
	command := app.NewCommand()
	if err := command.Execute(); err != nil {
		klog.Errorf("command execution failed with error: %+v", err)
		os.Exit(1)
	}
}

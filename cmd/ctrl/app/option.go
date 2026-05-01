package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/BuMaRen/mesh/pkg/ctrl"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

type Options struct {
	Namespace   string
	PodName     string
	ctrlOptions *ctrl.Options
}

func NewOptions() *Options {
	return &Options{
		ctrlOptions: ctrl.NewOptions(),
	}
}

func (o *Options) AddFlags(command *cobra.Command) {
	command.Flags().StringVar(&o.Namespace, "namespace", "", "The namespace of the pod")
	command.Flags().StringVar(&o.PodName, "pod-name", "", "The name of the pod")
	o.ctrlOptions.AddFlags(command)
}

func (o *Options) Run() {
	rootContext := WithSignalCatch(context.Background())
	workingContext, cancel := context.WithCancel(rootContext)
	defer cancel()
	ctrl.StartUp(workingContext, o.ctrlOptions)
}

func WithSignalCatch(root context.Context) context.Context {
	ctx, cancel := context.WithCancel(root)
	ch := make(chan os.Signal, 1)
	signal.Reset(syscall.SIGTERM, syscall.SIGINT)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		// Listen for OS signals and call cancel() when received
		<-ch
		klog.Infof("Received shutdown signal, initiating graceful shutdown...")
		cancel()
		<-ch
		klog.Infof("Received second shutdown signal, forcing exit...")
		os.Exit(1)
	}()
	return ctx
}

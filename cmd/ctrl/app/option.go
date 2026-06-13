package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/BuMaRen/mesh/internal/ctrl"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

type Options struct {
	ctrlOptions *ctrl.Options
}

func NewOptions() *Options {
	return &Options{
		ctrlOptions: ctrl.NewOptions(),
	}
}

func (o *Options) CtrlOptions() *ctrl.Options {
	return o.ctrlOptions
}

func (o *Options) AddFlags(command *cobra.Command) {
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

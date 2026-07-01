package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/klog/v2"
)

func WithSignalCatch(root context.Context) context.Context {
	ctx, cancel := context.WithCancel(root)
	ch := make(chan os.Signal, 1)
	signal.Reset(syscall.SIGTERM, syscall.SIGINT)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-ch
		klog.Warning("[Signal] graceful shutdown in progress...")
		cancel()
		<-ch
		klog.Warning("[Signal] received second shutdown signal, forcing exit...")
		os.Exit(1)
	}()
	return ctx
}

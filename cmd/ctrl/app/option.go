package app

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/BuMaRen/mesh/pkg/ctrl/logic"
	"k8s.io/klog/v2"
)

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

func Serve(l *logic.Logic) error {
	rootContext := WithSignalCatch(context.Background())
	workingContext, cancel := context.WithCancel(rootContext)
	defer cancel()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		l.WatchEndpointSliceOrDie(workingContext)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := l.ServeGrpcOrDie(workingContext); err != nil {
			klog.Errorf("Failed to serve gRPC: %v", err)
			cancel()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := l.ServeHttpsOrDie(workingContext); err != nil {
			klog.Errorf("Failed to serve HTTPS: %v", err)
			cancel()
		}
	}()
	wg.Wait()
	return nil
}

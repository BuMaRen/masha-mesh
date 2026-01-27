package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BuMaRen/masha-mesh/pkg/controller"
	"github.com/BuMaRen/masha-mesh/pkg/server"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

var (
	port      = flag.Int("port", 8080, "HTTP server port")
	namespace = flag.String("namespace", "", "Namespace to watch (empty for all namespaces)")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Failed to create in-cluster config: %v", err)
	}

	// Create Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	klog.Info("Control-face starting...")
	klog.Infof("Watching namespace: %s", getNamespace(*namespace))

	// Create service discovery controller
	ctrl := controller.NewServiceController(clientset, *namespace)

	// Start the controller
	go func() {
		if err := ctrl.Run(ctx); err != nil {
			klog.Fatalf("Failed to run controller: %v", err)
		}
	}()

	// Create and start HTTP server
	srv := server.NewServer(*port, ctrl)
	go func() {
		if err := srv.Start(); err != nil {
			klog.Fatalf("Failed to start server: %v", err)
		}
	}()

	klog.Infof("Control-face started successfully on port %d", *port)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	klog.Info("Shutting down control-face...")
	cancel()

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		klog.Errorf("Server shutdown error: %v", err)
	}

	klog.Info("Control-face stopped")
}

func getNamespace(ns string) string {
	if ns == "" {
		return "all"
	}
	return ns
}

package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/BuMaRen/masha-mesh/pkg/api"
	"github.com/BuMaRen/masha-mesh/pkg/proxy"
)

type SidecarConfig struct {
	Mode              string // "l4" or "l7"
	ListenAddr        string
	TargetAddr        string // For L4 mode
	ControlPlaneAddr  string
	ServiceName       string
	EnableControlPlane bool
}

func main() {
	config := &SidecarConfig{}
	
	flag.StringVar(&config.Mode, "mode", "l7", "Proxy mode: l4 or l7")
	flag.StringVar(&config.ListenAddr, "listen", ":8000", "Listen address")
	flag.StringVar(&config.TargetAddr, "target", "localhost:8001", "Target address (for L4 mode)")
	flag.StringVar(&config.ControlPlaneAddr, "control-plane", "localhost:9090", "Control plane address")
	flag.StringVar(&config.ServiceName, "service", "example-service", "Service name")
	flag.BoolVar(&config.EnableControlPlane, "enable-cp", false, "Enable control plane integration")
	flag.Parse()
	
	log.Printf("Starting Sidecar in %s mode...", config.Mode)
	log.Printf("Listen address: %s", config.ListenAddr)
	
	if config.Mode == "l4" {
		runL4Proxy(config)
	} else {
		runL7Proxy(config)
	}
}

func runL4Proxy(config *SidecarConfig) {
	log.Printf("L4 Mode: Forwarding to %s", config.TargetAddr)
	
	proxy := proxy.NewL4Proxy(config.ListenAddr, config.TargetAddr)
	
	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		log.Println("Shutting down L4 proxy...")
		os.Exit(0)
	}()
	
	if err := proxy.Start(); err != nil {
		log.Fatalf("L4 Proxy error: %v", err)
	}
}

func runL7Proxy(config *SidecarConfig) {
	var l7proxy *proxy.L7Proxy
	var err error
	
	if config.EnableControlPlane {
		log.Println("L7 Mode with Control Plane integration")
		
		// Connect to control plane
		conn, err := grpc.NewClient(config.ControlPlaneAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("Failed to connect to control plane: %v", err)
		}
		defer conn.Close()
		
		client := pb.NewControlPlaneClient(conn)
		
		// Get initial configuration
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := client.GetServiceConfig(ctx, &pb.ServiceConfigRequest{
			ServiceName: config.ServiceName,
		})
		cancel()
		
		if err != nil {
			log.Fatalf("Failed to get service config: %v", err)
		}
		
		log.Printf("Received configuration for service %s: %v", resp.ServiceName, resp.Endpoints)
		
		// Create proxy with initial backends
		backends := make([]string, len(resp.Endpoints))
		for i, ep := range resp.Endpoints {
			if !strings.HasPrefix(ep, "http://") && !strings.HasPrefix(ep, "https://") {
				backends[i] = "http://" + ep
			} else {
				backends[i] = ep
			}
		}
		
		l7proxy, err = proxy.NewL7Proxy(config.ListenAddr, backends)
		if err != nil {
			log.Fatalf("Failed to create L7 proxy: %v", err)
		}
		
		// Start watching for config updates
		go watchConfigUpdates(client, config.ServiceName, l7proxy)
		
	} else {
		log.Printf("L7 Mode: Forwarding to %s", config.TargetAddr)
		
		// Static configuration
		target := config.TargetAddr
		if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
			target = "http://" + target
		}
		
		l7proxy, err = proxy.NewL7Proxy(config.ListenAddr, []string{target})
		if err != nil {
			log.Fatalf("Failed to create L7 proxy: %v", err)
		}
	}
	
	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		log.Println("Shutting down L7 proxy...")
		os.Exit(0)
	}()
	
	if err := l7proxy.Start(); err != nil {
		log.Fatalf("L7 Proxy error: %v", err)
	}
}

func watchConfigUpdates(client pb.ControlPlaneClient, serviceName string, proxy *proxy.L7Proxy) {
	for {
		stream, err := client.StreamConfig(context.Background(), &pb.ServiceConfigRequest{
			ServiceName: serviceName,
		})
		if err != nil {
			log.Printf("Failed to start config stream: %v, retrying in 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}
		
		for {
			resp, err := stream.Recv()
			if err != nil {
				log.Printf("Stream error: %v, reconnecting...", err)
				break
			}
			
			log.Printf("Received config update: %v", resp.Endpoints)
			
			// Update proxy backends
			backends := make([]string, len(resp.Endpoints))
			for i, ep := range resp.Endpoints {
				if !strings.HasPrefix(ep, "http://") && !strings.HasPrefix(ep, "https://") {
					backends[i] = "http://" + ep
				} else {
					backends[i] = ep
				}
			}
			
			if err := proxy.UpdateBackends(backends); err != nil {
				log.Printf("Failed to update backends: %v", err)
			}
		}
		
		time.Sleep(5 * time.Second)
	}
}

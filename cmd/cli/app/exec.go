package app

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/BuMaRen/mesh/pkg/cli"
	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

func WithTarget(target string) ExecutorSetter {
	return func(e *Executor) {
		e.target = target
	}
}

func WithUid(uid string) ExecutorSetter {
	return func(e *Executor) {
		e.uid = uid
	}
}

func WithPort(port int) ExecutorSetter {
	return func(e *Executor) {
		e.port = port
	}
}

func WithAddress(address string) ExecutorSetter {
	return func(e *Executor) {
		e.address = address
	}
}

func NewExecutor(fns ...ExecutorSetter) *Executor {
	executor := &Executor{}
	for _, fn := range fns {
		fn(executor)
	}
	return executor
}

type ExecutorSetter func(*Executor)

type Executor struct {
	target     string
	uid        string
	port       int
	address    string
	meshClient *cli.MeshClient
}

func (e *Executor) Complete(options *Options) {
	// 与 options 进行对接必要的参数
	e.target = options.target
	e.uid = options.uid

	// 听取动态配置必要的参数
	e.address = options.address
}

// 一个 server，用于监听动态配置的变更从而影响sidecar的行为
func (e *Executor) configServer(ctx context.Context) {
	if _, _, err := net.SplitHostPort(e.address); err != nil {
		klog.Errorf("Invalid address: %s\n", e.address)
		panic(fmt.Sprintf("Invalid address: %s", e.address))
	}
	listener, err := net.Listen("tcp", e.address)
	if err != nil {
		klog.Errorf("Failed to start config server: %v\n", err)
		panic(err)
	}

	configContext, cancel := context.WithCancel(ctx)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-configContext.Done()
		klog.Info("Shutting down config server...")
		if err := listener.Close(); err != nil {
			klog.Errorf("Failed to close listener: %v\n", err)
		}
		klog.Info("Shutting down config server successfully")
	}()

	router := gin.Default()
	// 取消订阅某个服务
	router.DELETE("/service/:svc-name", func(c *gin.Context) {
		serviceName := c.Param("svc-name")
		klog.Infof("Received request to delete service: %s\n", serviceName)
		if err := e.meshClient.Unsubscribe(configContext, serviceName); err != nil {
			klog.Errorf("Failed to unsubscribe from service: %s, error: %v\n", serviceName, err)
			c.JSON(500, gin.H{"message": fmt.Sprintf("Failed to unsubscribe from service: %s", serviceName)})
			return
		}
		c.JSON(200, gin.H{"message": fmt.Sprintf("Unsubscribed from service: %s", serviceName)})
	})
	// 订阅新的服务
	router.POST("/service/:svc-name", func(c *gin.Context) {
		serviceName := c.Param("svc-name")
		klog.Infof("Received request to add service: %s\n", serviceName)
		go e.meshClient.Subscribe(configContext, serviceName)
		c.JSON(200, gin.H{"message": fmt.Sprintf("Subscribed to service: %s", serviceName)})
	})

	if err := router.RunListener(listener); err != nil {
		klog.Warningf("Failed to run listener: %v\n", err)
	}
	cancel()
	wg.Wait()
}

func (e *Executor) routingServer(ctx context.Context) {
	// sidecar 的路由逻辑入口
}

func (e *Executor) Run(ctx context.Context) {
	meshClient := cli.NewMeshClient()
	meshClient.Connect(e.target)
	e.meshClient = meshClient
	// // 启动时默认不订阅任何服务，等待外部请求来订阅
	// meshClient.Subscribe(context.Background(), "mesh-ctrl")

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		e.configServer(ctx)
	}()

	e.routingServer(ctx)
	wg.Wait()
}

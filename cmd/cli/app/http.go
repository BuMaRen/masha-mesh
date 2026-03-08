package app

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

// http server监听来自于外部的动态配置变更
// 通过meshClient影响到serviceCache中的缓存的endpoint信息

// 一个 server，用于监听动态配置的变更从而影响sidecar的行为
func (e *Executor) configServer(ctx context.Context) {
	listener := splitHostPort(e.address)
	configContext, cancel := context.WithCancel(ctx)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-configContext.Done()
		klog.Info("Shutting down config server...")
		if err := listener.Close(); err != nil {
			klog.Errorf("Failed to close listener: %v\n", err)
			return
		}
		klog.Info("Shutting down config server successfully")
	}()

	router := gin.Default()
	// 取消订阅某个服务
	router.DELETE("/service/:svc-name", e.handlerChainMake(configContext, e.handleDeleteServiceEvent)...)
	// 订阅新的服务
	router.POST("/service/:svc-name", e.handlerChainMake(configContext, e.handleAddServiceEvent)...)

	if err := router.RunListener(listener); err != nil {
		klog.Warningf("Failed to run listener: %v\n", err)
	}
	cancel()
	wg.Wait()
}

func (e *Executor) handlerChainMake(parentContext context.Context, fn func(context.Context, string) (int, error)) []gin.HandlerFunc {
	handlers := []gin.HandlerFunc{}
	var serviceName string
	handlers = append(handlers, func(c *gin.Context) {
		serviceName = c.Param("svc-name")
	})
	handlers = append(handlers, func(c *gin.Context) {
		statusCode, err := fn(parentContext, serviceName)
		if statusCode != http.StatusOK {
			c.JSON(statusCode, gin.H{"message": err.Error()})
			return
		}
		c.JSON(statusCode, gin.H{"message": fmt.Sprintf("operation on service %s successfully", serviceName)})
	})
	return handlers
}

func (e *Executor) handleAddServiceEvent(parentContext context.Context, serviceName string) (int, error) {
	if _, existed := e.svcContext.GetCancel(serviceName); existed {
		klog.Infof("Service %s is already subscribed\n", serviceName)
		return http.StatusInternalServerError, fmt.Errorf("service %s is already subscribed", serviceName)
	}

	klog.Infof("Received request to add service: %s\n", serviceName)

	ctx := e.svcContext.NewServiceContext(parentContext, serviceName)
	go e.meshClient.Subscribe(ctx, serviceName)
	return http.StatusOK, nil
}

func (e *Executor) handleDeleteServiceEvent(parentContext context.Context, serviceName string) (int, error) {
	klog.Infof("Received request to delete service: %s\n", serviceName)
	if cancel, existed := e.svcContext.GetCancel(serviceName); existed {
		cancel()
		return http.StatusOK, nil
	}
	return http.StatusInternalServerError, fmt.Errorf("subscription of service %s not found", serviceName)
}

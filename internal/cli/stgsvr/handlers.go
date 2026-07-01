package stgsvr

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

// 用于策略的动态更新

func (s *Server) attachHandlers(ctx context.Context) {
	s.engine.POST("/service/:svc-name", s.subscribeChain(ctx, "svc-name")...)
	s.engine.DELETE("/service/:svc-name", s.unsubscribeChain(ctx, "svc-name")...)
}

func (s *Server) subscribeChain(ctx context.Context, key string) []gin.HandlerFunc {
	handlers := []gin.HandlerFunc{}
	handlers = append(handlers, func(c *gin.Context) {
		serviceName := c.Param(key)
		if serviceName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "serviceName is required"})
			c.Abort()
			return
		}
		c.Set(key, serviceName)
	})
	handlers = append(handlers, func(c *gin.Context) {
		serviceName := c.GetString(key)
		if _, existed := s.svcContext.GetCancel(serviceName); existed {
			c.JSON(http.StatusBadRequest, gin.H{"error": "service already existed, unsubscribe first"})
			return
		}
		if err := s.subscribe(ctx, serviceName); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "subscribe successfully"})
	})
	return handlers
}

func (s *Server) unsubscribeChain(_ context.Context, key string) []gin.HandlerFunc {
	handlers := []gin.HandlerFunc{}
	handlers = append(handlers, func(c *gin.Context) {
		if serviceName := c.Param(key); serviceName != "" {
			c.Set(key, serviceName)
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "serviceName is required"})
		c.Abort()
	})
	handlers = append(handlers, func(c *gin.Context) {
		serviceName := c.GetString(key)
		if err := s.unsubscribe(serviceName); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "unsubscribe successfully"})
	})
	return handlers
}

func (s *Server) subscribe(parent context.Context, serviceName string) error {
	ctx := s.svcContext.NewServiceContext(parent, serviceName)
	return s.client.Subscribe(ctx, serviceName)
}

func (s *Server) unsubscribe(serviceName string) error {
	cancel, existed := s.svcContext.GetCancel(serviceName)
	if !existed {
		klog.Warningf("[StgSvr] service %s not found", serviceName)
		return fmt.Errorf("service %v not found", serviceName)
	}
	cancel()
	return s.client.Unsubscribe(context.TODO(), serviceName)
}

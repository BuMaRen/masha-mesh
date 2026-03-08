package httpserver

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *HttpServer) attachHandlers(ctx context.Context) {
	s.engine.POST("/service/:svc-name", s.subscribeChain(ctx, "svc-name")...)
	s.engine.DELETE("/service/:svc-name", s.unsubscribeChain(ctx, "svc-name")...)
}

func (s *HttpServer) subscribeChain(ctx context.Context, key string) []gin.HandlerFunc {
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
		ctx := s.svcContext.NewServiceContext(ctx, serviceName)
		if err := s.subscribe(ctx, serviceName); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "subscribe successfully"})
	})
	return handlers
}

func (s *HttpServer) unsubscribeChain(_ context.Context, key string) []gin.HandlerFunc {
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

package main

import (
	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

func main() {
	engine := gin.Default()
	engine.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	klog.Info("Starting test HTTP server on :8081")
	engine.Run(":8081")
}

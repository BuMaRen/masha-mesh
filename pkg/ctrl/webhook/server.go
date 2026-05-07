package webhook

import (
	"context"
	"net/http"

	"github.com/BuMaRen/mesh/pkg/ctrl/data"
	"github.com/BuMaRen/mesh/pkg/ctrl/resources"
	"github.com/BuMaRen/mesh/pkg/ctrl/utils"
	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

type WebhookServer struct {
	engine         *gin.Engine
	containerCache data.Cache
}

func NewWebhookServer(containerCache data.Cache) *WebhookServer {
	engine := gin.Default()
	return &WebhookServer{
		engine:         engine,
		containerCache: containerCache,
	}
}

func (s *WebhookServer) getContainerCache(name string) *resources.Container {
	cache, exist := s.containerCache.GetCache(name)
	if !exist {
		return nil
	}
	return cache.(*resources.Container)
}

func (s *WebhookServer) Run(ctx context.Context, opts *Options) error {
	listener := utils.NewListenerOrDie("tcp", opts.address)
	defer listener.Close()

	engine := gin.Default()
	s.Aggregation(engine, opts.imageTag, opts.commands)
	httpSvr := &http.Server{
		Handler: engine.Handler(),
	}

	go func() {
		<-ctx.Done()
		err := httpSvr.Close()
		klog.Infof("Shutting down HTTPS server, error: %v", err)
	}()

	return httpSvr.ServeTLS(listener, opts.certFile, opts.keyFile)
}

func (s *WebhookServer) Start(address string) error {
	return s.engine.Run(address)
}

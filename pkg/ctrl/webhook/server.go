package webhook

import (
	"context"
	"net/http"
	"time"

	"github.com/BuMaRen/mesh/pkg/ctrl/data"
	"github.com/BuMaRen/mesh/pkg/ctrl/resources"
	"github.com/BuMaRen/mesh/pkg/ctrl/utils"
	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

type WebhookServer struct {
	containerCache data.Cache
	injectionLabel string
}

type WebOption func(*WebhookServer)

func WithInjectionLabel(label string) WebOption {
	return func(s *WebhookServer) {
		s.injectionLabel = label
	}
}

func NewWebhookServer(containerCache data.Cache, opts ...WebOption) *WebhookServer {
	server := &WebhookServer{
		containerCache: containerCache,
	}
	for _, opt := range opts {
		opt(server)
	}
	return server
}

func (s *WebhookServer) getContainerCache(name string) *resources.Container {
	cache, exist := s.containerCache.GetCache(name)
	if !exist {
		return nil
	}
	return cache.(*resources.Container)
}

func (s *WebhookServer) Run(ctx context.Context, opts *Options) error {
	s.injectionLabel = opts.injectionLabel

	listener := utils.NewListenerOrDie("tcp", opts.address)
	defer listener.Close()

	engine := gin.Default()
	s.Aggregation(engine)
	httpSvr := &http.Server{
		Handler: engine.Handler(),
	}

	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := httpSvr.Shutdown(ctx)
		klog.Infof("Shutting down HTTPS server, error: %v", err)
	}()

	return httpSvr.ServeTLS(listener, opts.certFile, opts.keyFile)
}

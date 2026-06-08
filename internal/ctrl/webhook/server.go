package webhook

import (
	"context"
	"net/http"
	"time"

	"github.com/BuMaRen/mesh/pkg/cache"
	"github.com/BuMaRen/mesh/pkg/utils"
	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type WebhookServer struct {
	kv    *cache.GeneralCache[*corev1.Container]
	label string
}

func NewWebhookServer(kv *cache.GeneralCache[*corev1.Container], label string) *WebhookServer {
	return &WebhookServer{
		kv:    kv,
		label: label,
	}
}

func (s *WebhookServer) Run(ctx context.Context, opts *Options) error {
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

package webhook

import (
	"context"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

type WebhookServer struct {
	engine *gin.Engine
}

func NewWebhookServer() *WebhookServer {
	engine := gin.Default()
	return &WebhookServer{
		engine: engine,
	}
}

func (s *WebhookServer) Run(ctx context.Context, opts *Options) error {
	listener, err := net.Listen("tcp", opts.address)
	if err != nil {
		return err
	}
	defer listener.Close()

	engine := gin.Default()
	aggregate(ctx, engine)
	httpSvr := &http.Server{
		Handler: engine.Handler(),
	}

	go func() {
		<-ctx.Done()
		err = httpSvr.Close()
		klog.Infof("Shutting down HTTPS server, error: %v", err)
	}()

	return httpSvr.ServeTLS(listener, opts.certFile, opts.keyFile)
}

func (s *WebhookServer) Start(address string) error {
	return s.engine.Run(address)
}

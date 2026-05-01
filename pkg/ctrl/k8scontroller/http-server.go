package k8scontroller

import (
	"context"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

func aggregate(ctx context.Context, engine *gin.Engine) {}

func ControllerRun(ctx context.Context, opts *Options) error {
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

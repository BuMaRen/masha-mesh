package logic

import (
	"context"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

type HttpsServer struct {
	address  string
	certFile string
	keyFile  string

	aggregator func(*gin.Engine)
}

func (s *HttpsServer) Serve(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return err
	}
	defer listener.Close()
	engine := gin.Default()
	s.aggregator(engine)
	httpSvr := &http.Server{
		Handler: engine.Handler(),
	}

	go func() {
		<-ctx.Done()
		err = httpSvr.Close()
		klog.Infof("Shutting down HTTPS server, error: %v", err)
	}()

	return httpSvr.ServeTLS(listener, s.certFile, s.keyFile)
}

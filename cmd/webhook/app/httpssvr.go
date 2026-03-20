package app

import (
	"context"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

type OptionFunc func(*HttpsServer)

type HttpsServer struct {
	address  string
	certFile string
	keyFile  string
}

func WithAddress(address string) OptionFunc {
	return func(s *HttpsServer) {
		s.address = address
	}
}

func WithCertAndKey(certFile, keyFile string) OptionFunc {
	return func(s *HttpsServer) {
		s.certFile = certFile
		s.keyFile = keyFile
	}
}

func NewHttpsServer(options ...OptionFunc) *HttpsServer {
	server := &HttpsServer{}
	for _, option := range options {
		option(server)
	}
	return server
}

func (s *HttpsServer) Complete(opts *Options) {
	s.address = opts.address
	s.certFile = opts.crt
	s.keyFile = opts.key
}

func (s *HttpsServer) Serve(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return err
	}
	defer listener.Close()
	engine := gin.Default()
	Aggregation(engine)
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

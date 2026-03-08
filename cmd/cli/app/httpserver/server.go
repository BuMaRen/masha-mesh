package httpserver

import (
	"context"
	"fmt"
	"net"

	"github.com/BuMaRen/mesh/pkg/cli"
	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

type OptionsFunc func(*HttpServer)

type HttpServer struct {
	address    string
	engine     *gin.Engine
	client     *cli.MeshClient
	svcContext *cli.ServiceContext
}

func WithAddress(address string) OptionsFunc {
	return func(s *HttpServer) {
		s.address = address
	}
}

func NewHttpServer(client *cli.MeshClient, serviceContext *cli.ServiceContext, opts ...OptionsFunc) *HttpServer {
	server := &HttpServer{
		client:     client,
		svcContext: serviceContext,
	}
	for _, opt := range opts {
		opt(server)
	}
	return server
}

func (s *HttpServer) Complete() {
	if _, _, err := net.SplitHostPort(s.address); err != nil {
		klog.Errorf("Invalid address: %s\n", s.address)
		panic(fmt.Sprintf("Invalid address: %s", s.address))
	}
	s.engine = gin.Default()
}

// Run 运行HTTP服务器
// Run 会阻塞直到 Context 取消或监听发生错误
func (s *HttpServer) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		klog.Errorf("Failed to start HTTP server: %v\n", err)
		return err
	}
	go func() {
		<-ctx.Done()
		klog.Info("Shutting down HTTP server...")
		if err := listener.Close(); err != nil {
			klog.Errorf("Failed to close listener: %v\n", err)
			return
		}
	}()

	s.attachHandlers(ctx)
	return s.engine.RunListener(listener)
}

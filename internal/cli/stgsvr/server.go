package stgsvr

import (
	"context"

	"github.com/BuMaRen/mesh/internal/cli/rpcclient"
	"github.com/BuMaRen/mesh/pkg/utils"
	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

type Server struct {
	engine     *gin.Engine
	client     *rpcclient.MeshClient
	svcContext *ServiceContext
}

func NewServer(client *rpcclient.MeshClient, serviceContext *ServiceContext) *Server {
	return &Server{
		engine:     gin.Default(),
		client:     client,
		svcContext: serviceContext,
	}
}

// Run 运行HTTP服务器
// Run 会阻塞直到 Context 取消或监听发生错误
func (s *Server) Run(ctx context.Context, opts *Options) error {
	listener := utils.NewListenerOrDie("tcp", opts.stgSvrAddress)
	go func() {
		<-ctx.Done()
		klog.Info("[StgSvr] shutting down HTTP server...")
		if err := listener.Close(); err != nil {
			klog.Errorf("[StgSvr] failed to close listener: %v", err)
			return
		}
	}()

	s.attachHandlers(ctx)
	return s.engine.RunListener(listener)
}

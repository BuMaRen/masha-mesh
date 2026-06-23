package ctrl

import (
	"context"
	"net/http"
	"time"

	"github.com/BuMaRen/mesh/pkg/utils"
	"k8s.io/klog/v2"
)

type HttpsServer struct {
	mux                     *http.ServeMux
	tlsCertFile             string
	tlsKeyFile              string
	address                 string
	gracefulShutdownTimeout time.Duration
}

func NewHttpsServer(opts *StartUpOptions) *HttpsServer {
	return &HttpsServer{
		mux:                     http.NewServeMux(),
		tlsCertFile:             opts.certFile,
		tlsKeyFile:              opts.keyFile,
		address:                 opts.address,
		gracefulShutdownTimeout: time.Duration(opts.gracefulShutdownTimeout) * time.Second,
	}
}

func (s *HttpsServer) RegisterHandler(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

func (s *HttpsServer) ServeTLS(ctx context.Context, stopCh chan struct{}) {
	listener := utils.NewListenerOrDie("tcp", s.address)
	defer listener.Close()

	httpSvr := &http.Server{
		Handler: s.mux,
	}
	go func() {
		select {
		case <-ctx.Done(): // 监听到上下文取消信号，开始优雅关闭服务器
			klog.Info("[HttpsServer] context canceled, shutting down...")
		case <-stopCh: // 监听到外部停止信号，开始优雅关闭服务器
			klog.Info("[HttpsServer] stop signal received, shutting down...")
		}
		stopCtx, cancel := context.WithTimeout(context.Background(), s.gracefulShutdownTimeout)
		defer cancel()
		err := httpSvr.Shutdown(stopCtx)
		if err != nil {
			// 如果优雅关闭失败，强制关闭服务器
			if closeErr := httpSvr.Close(); closeErr != nil {
				klog.Errorf("[HttpsServer] failed to force close: %v", closeErr)
				return
			}
			klog.Errorf("[HttpsServer] graceful shutdown failed: %v", err)
			return
		}
		klog.Info("[HttpsServer] graceful shutdown completed")
	}()

	if err := httpSvr.ServeTLS(listener, s.tlsCertFile, s.tlsKeyFile); err != nil && err != http.ErrServerClosed {
		klog.Errorf("[HttpsServer] failed to start: %v", err)
	}
	klog.Info("[HttpsServer] stopped")
}

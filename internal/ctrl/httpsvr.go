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

func NewHttpsServer(opts *Options) *HttpsServer {
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
			klog.Info("context canceled, shutting down HTTPS server...")
		case <-stopCh: // 监听到外部停止信号，开始优雅关闭服务器
			klog.Info("stop signal received, shutting down HTTPS server...")
		}
		stopCtx, cancel := context.WithTimeout(context.Background(), s.gracefulShutdownTimeout)
		defer cancel()
		err := httpSvr.Shutdown(stopCtx)
		if err != nil {
			// 如果优雅关闭失败，强制关闭服务器
			if closeErr := httpSvr.Close(); closeErr != nil {
				klog.Errorf("Failed to force close HTTPS server, error: %v", closeErr)
				return
			}
			klog.Errorf("HTTPS server graceful shutdown failed: %v", err)
			return
		}
		klog.Info("HTTPS server graceful shutdown completed")

	if err := httpSvr.ServeTLS(listener, s.tlsCertFile, s.tlsKeyFile); err != nil && err != http.ErrServerClosed {
		klog.Errorf("Failed to start HTTPS server, error: %v", err)
	}
	klog.Info("HTTPS server stopped")
}

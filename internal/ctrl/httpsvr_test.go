package ctrl

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/BuMaRen/mesh/internal/ctrl/hooks"
)

func TestHttpsServer(t *testing.T) {
	t.Run("graceful shutdown", func(t *testing.T) {
		_, currentFile, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("failed to get current file path")
		}
		repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
		certFile := filepath.Join(repoRoot, "build", "certs", "tls.crt")
		keyFile := filepath.Join(repoRoot, "build", "certs", "tls.key")

		opts := &StartUpOptions{
			certFile:                certFile,
			keyFile:                 keyFile,
			address:                 "localhost:8443",
			gracefulShutdownTimeout: 15,
		}

		caPem, err := os.ReadFile(certFile)
		if err != nil {
			t.Fatalf("Failed to read CA cert: %v", err)
		}
		rootCAs := x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(caPem) {
			t.Fatal("Failed to append CA cert to cert pool")
		}

		stopCh := make(chan struct{})
		httpSvr := NewHttpsServer(opts)
		httpSvr.RegisterHandler("/preStop", hooks.NewPreStopHandler(stopCh))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			httpSvr.ServeTLS(ctx, stopCh)
			cancel()
		}()

		// 配置 HTTP 客户端，使用 build/certs 中的证书做真实 TLS 校验
		client := &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
					RootCAs:    rootCAs,
					ServerName: "localhost",
				},
			},
		}

		// 等待服务器启动，轮询直到服务器就绪
		var resp *http.Response
		for i := 0; i < 50; i++ {
			resp, err = client.Post("https://"+opts.address+"/preStop", "application/json", nil)
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if err != nil {
			t.Fatalf("Failed to send preStop request after retries: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status code 200, got %d", resp.StatusCode)
		}

		// 等待一段时间，确保服务器有机会完成优雅关闭
		select {
		case <-ctx.Done():
			// 上下文已取消，说明服务器已成功关闭
		case <-time.After(20 * time.Second):
			t.Fatal("HTTPS server did not shut down gracefully within the expected time")
		}
	})
}

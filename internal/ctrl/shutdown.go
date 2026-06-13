package ctrl

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

type preStopHandler struct {
	stop chan struct{}
	once sync.Once
}

func (h *preStopHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	// 防止处理慢、多个请求导致多次调用 Execute，使用 sync.Once 确保只执行一次
	h.once.Do(func() {
		close(h.stop)
	})
}

var _ http.Handler = (*preStopHandler)(nil)

type Shutdown struct {
	address  string
	certFile string
	timeout  time.Duration
}

func NewShutdown(opt *Options) *Shutdown {
	return &Shutdown{
		address:  opt.address,
		certFile: opt.certFile,
		timeout:  time.Duration(opt.gracefulShutdownTimeout) * time.Second,
	}
}

func addressValidate(address string) (string, bool) {
	// 判断地址是否合法，":port" 的情况补全 localhost，必须包含端口号
	if address == "" {
		return "", false
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", false
	}
	if host == "" {
		host = "localhost"
	}
	return net.JoinHostPort(host, port), true
}

func (s *Shutdown) Execute() {
	address, valid := addressValidate(s.address)
	if !valid {
		klog.Errorf("Invalid address: %s", s.address)
		return
	}

	caPem, err := os.ReadFile(s.certFile)
	if err != nil {
		klog.Errorf("Failed to read CA cert: %v", err)
		return
	}
	rootCAs := x509.NewCertPool()
	if !rootCAs.AppendCertsFromPEM(caPem) {
		klog.Errorf("Failed to append CA cert to cert pool")
		return
	}
	client := &http.Client{
		Timeout: s.timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				RootCAs:    rootCAs,
				ServerName: "localhost",
			},
		},
	}

	resp, err := client.Post("https://"+address+"/preStop", "application/json", nil)
	if err != nil {
		klog.Errorf("Failed to send preStop request: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		klog.Errorf("Expected status code 200, got %d", resp.StatusCode)
		return
	}
}

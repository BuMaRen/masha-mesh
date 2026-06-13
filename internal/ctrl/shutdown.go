package ctrl

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"time"

	"k8s.io/klog/v2"
)

type preStopHandler struct {
	stop chan struct{}
}

func (h *preStopHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	close(h.stop)
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

func (s *Shutdown) Execute() {
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
	resp, err := client.Post("https://"+s.address+"/preStop", "application/json", nil)
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

package ctrl

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"os"
	"time"

	"k8s.io/klog/v2"
)

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

func Shutdown(opt *ShutdownOptions) {
	address, valid := addressValidate(opt.address)
	if !valid {
		klog.Errorf("[Shutdown] invalid address: %s", opt.address)
		return
	}

	caPem, err := os.ReadFile(opt.certFile)
	if err != nil {
		klog.Errorf("[Shutdown] failed to read CA cert: %v", err)
		return
	}
	rootCAs := x509.NewCertPool()
	if !rootCAs.AppendCertsFromPEM(caPem) {
		klog.Errorf("[Shutdown] failed to append CA cert to cert pool")
		return
	}
	host, _, _ := net.SplitHostPort(address)
	client := &http.Client{
		Timeout: time.Duration(opt.gracefulShutdownTimeout) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				RootCAs:    rootCAs,
				ServerName: host,
			},
		},
	}

	resp, err := client.Post("https://"+address+"/preStop", "application/json", nil)
	if err != nil {
		klog.Errorf("[Shutdown] failed to send preStop request: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		klog.Errorf("[Shutdown] expected status code 200, got %d", resp.StatusCode)
		return
	}
}

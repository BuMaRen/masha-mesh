package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/BuMaRen/mesh/internal/cli/proxyconfig"
	"k8s.io/klog/v2"
)

type httpTransport struct {
	listener *Listener
}

func (t *httpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	svrHost, svrPort, isService := parseHost(req.Host)
	klog.Infof("Proxy: Received request for host: %s (service: %s, port: %s, isService: %v)",
		req.Host, svrHost, svrPort, isService)

	// IP-based requests always pass through
	if !isService {
		return t.passThrough(req, "IP-based request, passing through to %s", req.Host)
	}

	// Check if HTTP proxy is enabled
	if t.listener.config == nil || !t.listener.config.HTTP.Enabled {
		return t.passThrough(req, "HTTP proxy disabled, passing through to %s", req.Host)
	}

	// Check if service has proxy configuration
	proxyRule, hasProxyConfig := t.listener.config.HTTP.Proxies[svrHost]
	if !hasProxyConfig {
		return t.passThrough(req, "No config for service %s, passing through to %s", svrHost, req.Host)
	}

	// Check if load balancing is enabled
	if !proxyRule.LoadBalance {
		return t.passThrough(req, "Load balancing disabled for %s, passing through to %s", svrHost, req.Host)
	}

	// Load balance across endpoints
	return t.loadBalance(req, svrHost, svrPort, proxyRule)
}

func (t *httpTransport) passThrough(req *http.Request, logFormat string, args ...interface{}) (*http.Response, error) {
	req.URL.Scheme = "http"
	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}
	klog.Infof("Proxy: "+logFormat, args...)
	return http.DefaultTransport.RoundTrip(req)
}

func (t *httpTransport) loadBalance(req *http.Request, service, port string, rule proxyconfig.ProxyRuleConfig) (*http.Response, error) {
	body, err := readBody(req)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	endpoints := t.getEndpoints(service)
	klog.Infof("Proxy: Load balancing across %d endpoints for service %s", len(endpoints), service)

	for _, endpoint := range endpoints {
		// Check circuit breaker
		if rule.UseBreaker && t.listener.breaker != nil {
			if !t.listener.breaker.Allowed(endpoint) {
				klog.Warningf("Proxy: Endpoint %s circuit broken, skipping", endpoint)
				continue
			}
		}

		klog.Infof("Proxy: Trying endpoint %s:%s", endpoint, port)
		resp, err := t.sendRequest(req, body, endpoint, port)

		// Record result in circuit breaker
		if rule.UseBreaker && t.listener.breaker != nil {
			if err != nil {
				t.recordFailure(endpoint, err)
				continue
			}
			t.listener.breaker.RecordSuccess(endpoint)
		}

		if err != nil {
			continue
		}

		klog.Infof("Proxy: Successfully proxied to %s:%s", endpoint, port)
		return resp, nil
	}

	return nil, fmt.Errorf("all endpoints failed for service %s", service)
}

func (t *httpTransport) sendRequest(req *http.Request, body []byte, ip, port string) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reqCopy := req.Clone(ctx)
	reqCopy.URL.Scheme = "http"
	reqCopy.Host = net.JoinHostPort(ip, port)
	reqCopy.URL.Host = reqCopy.Host
	reqCopy.RequestURI = ""
	reqCopy.Body = io.NopCloser(bytes.NewReader(body))

	resp, err := http.DefaultClient.Do(reqCopy)
	if err != nil {
		klog.V(4).Infof("request to %s failed: %v", reqCopy.Host, err)
		return nil, err
	}

	// Treat 5xx as failure
	if resp.StatusCode >= 500 {
		resp.Body.Close()
		return nil, fmt.Errorf("server error: %d", resp.StatusCode)
	}

	return resp, nil
}

func (t *httpTransport) recordFailure(endpoint string, err error) {
	errStr := err.Error()
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		t.listener.breaker.RecordTimeout(endpoint)
		klog.Warningf("Proxy: Timeout for %s", endpoint)
	} else if strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "network") {
		t.listener.breaker.RecordNetworkFailure(endpoint)
		klog.Warningf("Proxy: Network failure for %s", endpoint)
	} else {
		t.listener.breaker.RecordBusinessFailure(endpoint)
		klog.Warningf("Proxy: Business failure for %s", endpoint)
	}
}

func (t *httpTransport) getEndpoints(service string) []string {
	result := []string{}
	eps := t.listener.meshClient.GetServiceIps(service)
	for _, ep := range eps {
		if len(ep) > 0 {
			result = append(result, ep[0])
		}
	}
	return result
}

func parseHost(host string) (hostname, port string, isService bool) {
	hostname = host
	port = "80"

	// Try standard host:port parsing first (handles IPv6 like [::1]:8080)
	if h, p, err := net.SplitHostPort(host); err == nil {
		hostname, port = h, p
	} else if strings.Count(host, ":") == 1 {
		// Fallback for "name:port" without IPv6 brackets
		parts := strings.SplitN(host, ":", 2)
		if len(parts) == 2 && parts[1] != "" {
			hostname, port = parts[0], parts[1]
		}
	}

	hostname = strings.Trim(hostname, "[]")
	isService = net.ParseIP(hostname) == nil
	return
}

func readBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return []byte{}, nil
	}
	defer req.Body.Close()
	return io.ReadAll(req.Body)
}

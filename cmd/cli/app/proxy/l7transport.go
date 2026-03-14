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

	"k8s.io/klog/v2"
)

// The transport used to perform proxy requests.
// If nil, http.DefaultTransport is used.

// RoundTrip executes a single HTTP transaction, returning
// a Response for the provided Request.
//
// RoundTrip should not attempt to interpret the response. In
// particular, RoundTrip must return err == nil if it obtained
// a response, regardless of the response's HTTP status code.
// A non-nil err should be reserved for failure to obtain a
// response. Similarly, RoundTrip should not attempt to
// handle higher-level protocol details such as redirects,
// authentication, or cookies.
//
// RoundTrip should not modify the request, except for
// consuming and closing the Request's Body. RoundTrip may
// read fields of the request in a separate goroutine. Callers
// should not mutate or reuse the request until the Response's
// Body has been closed.
//
// RoundTrip must always close the body, including on errors,
// but depending on the implementation may do so in a separate
// goroutine even after RoundTrip returns. This means that
// callers wanting to reuse the body for subsequent requests
// must arrange to wait for the Close call before doing so.
//
// The Request's URL and Header fields must be initialized.
func (l7 *L7Proxy) RoundTrip(req *http.Request) (*http.Response, error) {
	svrHost, svrPort, isService := serviceAsHost(req.Host)
	klog.Infof("L7 Proxy: Received request for host: %s, service: %s, port: %s", req.Host, svrHost, svrPort)
	if !isService {
		req.URL.Scheme = "http"
		if req.URL.Host == "" {
			req.URL.Host = req.Host
		}
		// source request has specified host ip
		return http.DefaultTransport.RoundTrip(req)
	}

	buffer, err := getBodyBytes(req)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %+v", err)
	}

	eps := l7.availableEndpoints(svrHost)
	for _, ep := range eps {
		klog.Infof("L7 Proxy: Attempting to proxy request to endpoint: %s:%s", ep, svrPort)
		resp, err := doCopiedRequest(req, buffer, ep, svrPort)
		if err != nil {
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("proxy failed to get response from all endpoints")
}

// 判断host是不是由service组成，是的话返回serviceName和True
func serviceAsHost(host string) (string, string, bool) {
	hostStr := host
	port := ""
	if strings.Contains(host, ":") {
		parts := strings.Split(host, ":")
		hostStr = parts[0]
		port = parts[1]
	}
	return hostStr, port, net.ParseIP(hostStr) == nil
}

func (l7 *L7Proxy) availableEndpoints(serviceName string) []string {
	result := []string{}
	eps := l7.meshClient.GetServiceIps(serviceName)
	for _, ep := range eps {
		result = append(result, ep[0])
	}
	return result
}

func getBodyBytes(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return []byte{}, nil
	}
	defer req.Body.Close()
	buffer, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

func doCopiedRequest(req *http.Request, body []byte, ip, port string) (*http.Response, error) {
	toCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reqCopy := req.Clone(toCtx)
	reqCopy.URL.Scheme = "http"
	reqCopy.Host = ip + ":" + port
	reqCopy.URL.Host = ip + ":" + port
	reqCopy.RequestURI = ""
	reqCopy.Body = io.NopCloser(bytes.NewReader(body))
	resp, err := http.DefaultClient.Do(reqCopy)
	if err != nil {
		klog.Warningf("request failed with error: %+v", err)
		return nil, err
	}
	if resp.StatusCode >= 500 {
		klog.Warningf("request to %s:%s failed with status code: %d", ip, port, resp.StatusCode)
		resp.Body.Close()
		return nil, fmt.Errorf("request to %s:%s failed with status code: %d", ip, port, resp.StatusCode)
	}
	return resp, nil
}

package l7

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

func (s *Server) RoundTrip(req *http.Request) (*http.Response, error) {
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

	eps := s.availableEndpoints(svrHost)
	for _, ep := range eps {
		klog.Infof("L7 Proxy: Attempting to proxy request to endpoint: %s:%s", ep, svrPort)
		// TODO：需要改造doCopiedRequest，按返回的错误用来指示breaker做增减
		resp, err := doCopiedRequest(req, buffer, ep, svrPort)
		if err != nil {
			// s.breakers.AddFailed(ep)
			continue
		}
		// TODO: 现在统计的是所有endpointslice的成功数量
		// s.breakers.AddSuccess(ep)
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

func (s *Server) availableEndpoints(serviceName string) []string {
	result := []string{}
	eps := s.client.GetServiceIps(serviceName)
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

	// TODO: promethus 统计放行、重试、内部失败、外部失败的请求数量
	// TODO：breaker 熔断器，失败率过高时熔断一段时间

	if resp.StatusCode >= 500 {
		klog.Warningf("request to %s:%s failed with status code: %d", ip, port, resp.StatusCode)
		resp.Body.Close()
		return nil, fmt.Errorf("request to %s:%s failed with status code: %d", ip, port, resp.StatusCode)
	}
	return resp, nil
}

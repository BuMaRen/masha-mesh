package routeserver

import (
	"context"
	"fmt"
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
func (l7 *L7RouteServer) RoundTrip(req *http.Request) (*http.Response, error) {
	svrHost, svrPort, isService := serviceAsHost(req.Host)
	if !isService {
		// source request has specified host ip
		return http.DefaultTransport.RoundTrip(req)
	}

	defer req.Body.Close()
	// get all endpoint ip of service
	// source request use service name as host
	eps := l7.availableEndpoints(svrHost)
	for _, ep := range eps {
		// TODO: body 是 nil 或者 body 是流，无法拷贝
		newBody, err := req.GetBody()
		if err != nil {
			// TODO: can error be returned?
			return nil, fmt.Errorf("proxy get body failed with error: %+v", err)
		}

		// 拷贝一个新的请求
		toCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		reqCopy := req.Clone(toCtx)
		reqCopy.URL.Scheme = "http"
		reqCopy.Host = ep + ":" + svrPort
		reqCopy.URL.Host = ep + ":" + svrPort
		reqCopy.Body = newBody
		resp, err := http.DefaultClient.Do(reqCopy)
		cancel()

		// 处理失败重试的场景
		if err != nil {
			klog.Warningf("request failed with error: %+v", err)
			continue
		}
		if resp.StatusCode < 500 {
			return resp, nil
		}
		resp.Body.Close()
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

func (l7 *L7RouteServer) availableEndpoints(serviceName string) []string {
	result := []string{}
	eps := l7.meshClient.GetServiceIps(serviceName)
	for _, ep := range eps {
		result = append(result, ep[0])
	}
	return result
}

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// ReadinessChecker 表示一个可检查就绪状态的组件
type ReadinessChecker interface {
	// Name 返回组件名称，用于日志和响应体中定位问题
	Name() string
	// Check 执行就绪检查，返回 nil 表示就绪，否则返回错误原因
	Check(ctx context.Context) error
}

// ReadyzHandler 聚合多个 ReadinessChecker，供 Kubernetes readinessProbe 调用
type ReadyzHandler struct {
	checkers []ReadinessChecker
	timeout  time.Duration
}

// NewReadyzHandler 创建 ReadyzHandler。
// timeout 是单次检查的超时时间，建议与 Kubernetes probe 的 timeoutSeconds 对齐。
func NewReadyzHandler(timeout time.Duration, checkers ...ReadinessChecker) http.Handler {
	return &ReadyzHandler{
		checkers: checkers,
		timeout:  timeout,
	}
}

func (h *ReadyzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	var failures []string
	for _, c := range h.checkers {
		if err := c.Check(ctx); err != nil {
			klog.Warningf("[readyz] checker %q not ready: %v", c.Name(), err)
			failures = append(failures, fmt.Sprintf("%s: %v", c.Name(), err))
		}
	}

	if len(failures) > 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "not ready\n%s\n", strings.Join(failures, "\n"))
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

// ---- 内置 checker 实现 ----

// KubeClientChecker 通过探测 API Server 的 /readyz 端点来判断 kube-client 是否可用
type KubeClientChecker struct {
	client kubernetes.Interface
}

func NewKubeClientChecker(client kubernetes.Interface) ReadinessChecker {
	return &KubeClientChecker{client: client}
}

func (c *KubeClientChecker) Name() string { return "kube-client" }

func (c *KubeClientChecker) Check(ctx context.Context) error {
	_, err := c.client.Discovery().RESTClient().Get().
		AbsPath("/readyz").
		DoRaw(ctx)
	return err
}

type DynamicClientChecker struct {
	client dynamic.Interface
}

func NewDynamicClientChecker(client dynamic.Interface) ReadinessChecker {
	return &DynamicClientChecker{client: client}
}

func (c *DynamicClientChecker) Name() string { return "dynamic-client" }

func (c *DynamicClientChecker) Check(ctx context.Context) error {
	// 检查 dynamic client 是否能成功列出某个核心资源（如 pods），以验证其基本功能
	_, err := c.client.Resource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}).
		List(ctx, metav1.ListOptions{Limit: 1})
	return err
}

// GrpcServerChecker 检查 gRPC 服务器是否已开始接受连接
// ready 由 GrpcServer 在成功监听后通过回调置为 true
type GrpcServerChecker struct {
	readyFn func() bool
}

// NewGrpcServerChecker 接受一个返回就绪状态的函数，由 GrpcServer 提供
func NewGrpcServerChecker(readyFn func() bool) ReadinessChecker {
	return &GrpcServerChecker{readyFn: readyFn}
}

func (c *GrpcServerChecker) Name() string { return "grpc-server" }

func (c *GrpcServerChecker) Check(_ context.Context) error {
	if !c.readyFn() {
		return fmt.Errorf("grpc server is not yet listening")
	}
	return nil
}

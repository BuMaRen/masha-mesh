package hooks

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

// LivenessChecker 表示一个可检查存活状态的组件
type LivenessChecker interface {
	// Name 返回组件名称，用于日志和响应体中定位问题
	Name() string
	// Check 执行存活检查，返回 nil 表示存活，否则返回错误原因
	Check(ctx context.Context) error
}

type HealthzHandler struct {
	checkers []LivenessChecker
	timeout  time.Duration
}

// NewHealthzHandler 创建 HealthzHandler。
// timeout 是单次检查的超时时间，建议与 Kubernetes probe 的 timeoutSeconds 对齐。
func NewHealthzHandler(timeout time.Duration, checkers ...LivenessChecker) http.Handler {
	return &HealthzHandler{
		checkers: checkers,
		timeout:  timeout,
	}
}

func (h *HealthzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	var failures []string
	for _, c := range h.checkers {
		if err := c.Check(ctx); err != nil {
			klog.V(2).Infof("[healthz] checker %q not healthy: %v", c.Name(), err)
			failures = append(failures, fmt.Sprintf("%s: %v", c.Name(), err))
		}
	}

	if len(failures) > 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "not healthy\n%s\n", strings.Join(failures, "\n"))
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

type DefaultHealthChecker struct {
	name string
}

func NewDefaultHealthChecker(name string) LivenessChecker {
	return &DefaultHealthChecker{name: name}
}

func (c *DefaultHealthChecker) Name() string { return c.name }

func (c *DefaultHealthChecker) Check(_ context.Context) error {
	return nil
}

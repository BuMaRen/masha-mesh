package handlers

import (
	"net/http"
	"sync"
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

func NewPreStopHandler(stop chan struct{}) http.Handler {
	return &preStopHandler{stop: stop}
}

var _ http.Handler = (*preStopHandler)(nil)

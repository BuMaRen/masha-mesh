package hooks

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
	h.once.Do(func() {
		close(h.stop)
	})
}

func NewPreStopHandler(stop chan struct{}) http.Handler {
	return &preStopHandler{stop: stop}
}

var _ http.Handler = (*preStopHandler)(nil)

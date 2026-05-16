package metrics

import (
	"context"
	"net/http"

	"github.com/BuMaRen/mesh/pkg/ctrl/utils"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
)

func RunMetricsServer(ctx context.Context, opts *Options) error {
	listener := utils.NewListenerOrDie("tcp", opts.address)
	defer listener.Close()

	httpSvr := &http.Server{
		Handler: promhttp.Handler(),
	}

	go func() {
		<-ctx.Done()
		err := httpSvr.Close()
		klog.Infof("Shutting down HTTP server, error: %v", err)
	}()

	return httpSvr.Serve(listener)
}

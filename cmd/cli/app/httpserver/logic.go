package httpserver

import (
	"context"
	"fmt"

	"k8s.io/klog/v2"
)

func (s *HttpServer) subscribe(parent context.Context, serviceName string) error {
	ctx := s.svcContext.NewServiceContext(parent, serviceName)
	return s.client.Subscribe(ctx, serviceName)
}

func (s *HttpServer) unsubscribe(serviceName string) error {
	cancel, existed := s.svcContext.GetCancel(serviceName)
	if !existed {
		klog.Errorf("service %v not found", serviceName)
		return fmt.Errorf("service %v not found", serviceName)
	}
	cancel()
	return s.client.Unsubscribe(context.TODO(), serviceName)
}

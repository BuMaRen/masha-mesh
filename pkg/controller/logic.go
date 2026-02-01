package controller

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/BuMaRen/mesh/pkg/api/mesh"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Controller struct {
	nameSpace string
	data      *Data
	syncer    *Syncer
}

func NewController(nameSpace string) *Controller {
	data := NewData()
	return &Controller{
		nameSpace: nameSpace,
		data:      data,
		syncer:    NewSyncer(data),
	}
}

func (l *Controller) RunAndServe(grpcPort int) error {
	if err := l.watchKubernetesEndpoints(); err != nil {
		return err
	}
	return l.runGrpcServer(grpcPort)
}

func (l *Controller) watchKubernetesEndpoints() error {
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	clientSet := kubernetes.NewForConfigOrDie(kubeConfig)
	esi := clientSet.DiscoveryV1().EndpointSlices(l.nameSpace)

	wi, err := esi.Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	ch := wi.ResultChan()
	go func() {
		for event := range ch {
			if err = l.data.Update(&event); err != nil {
				log.Printf("error updating data: %v\n", err)
			}
		}
	}()
	return nil
}

// RunServer 启动 gRPC 服务器
func (l *Controller) runGrpcServer(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	gSvr := grpc.NewServer()
	mesh.RegisterControlFaceServer(gSvr, l.syncer)
	return gSvr.Serve(listener)
}

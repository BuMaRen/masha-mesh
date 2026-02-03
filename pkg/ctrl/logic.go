package ctrl

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var _life = make(chan struct{})

func rootContext() context.Context {
	close(_life)

	stopCh := make(chan os.Signal, 2)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-stopCh
		cancel()
		<-stopCh
		os.Exit(1) // second signal. Exit directly.
	}()
	return ctx
}

type Logic struct {
	nameSpace string
	grpcPort  int
	storage   *EndpointSliceMap
}

func (l *Logic) watchEndpointSlicesOrDie(ctx context.Context) {
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}
	clientSet := kubernetes.NewForConfigOrDie(kubeConfig)
	esi := clientSet.DiscoveryV1().EndpointSlices(l.nameSpace)
	wi, err := esi.Watch(ctx, metav1.ListOptions{})
	if err != nil {
		panic(err)
	}
	ch := wi.ResultChan()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			l.storage.OnUpdate(&event)
		}
	}
}

func (l *Logic) serveGrpcOrDie(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", l.grpcPort))
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	syncer := NewSync(l.storage)
	grpcSvr := syncer.NewGrpcServer()
	go func() {
		<-ctx.Done()
		grpcSvr.GracefulStop()
	}()
	err = grpcSvr.Serve(listener)
	return err
}

func (l *Logic) Run() {

	ctx := rootContext()
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		l.watchEndpointSlicesOrDie(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := l.serveGrpcOrDie(ctx); err != nil {
			fmt.Printf("grpc server exit with err: %v\n", err)
		}
	}()

	wg.Wait()
	fmt.Println("controller exited")
}

func NewLogic(nameSpace string, grpcPort int) *Logic {
	return &Logic{
		nameSpace: nameSpace,
		grpcPort:  grpcPort,
		storage:   NewEndpointSliceMap(),
	}
}

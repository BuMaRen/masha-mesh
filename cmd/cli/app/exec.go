package app

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/BuMaRen/mesh/pkg/cli"
	"k8s.io/klog/v2"
)

func WithTarget(target string) ExecutorSetter {
	return func(e *Executor) {
		e.target = target
	}
}

func WithUid(uid string) ExecutorSetter {
	return func(e *Executor) {
		e.uid = uid
	}
}

func WithPort(port int) ExecutorSetter {
	return func(e *Executor) {
		e.port = port
	}
}

func WithAddress(address string) ExecutorSetter {
	return func(e *Executor) {
		e.address = address
	}
}

func NewExecutor(fns ...ExecutorSetter) *Executor {
	executor := &Executor{
		svcContext: NewServiceContext(),
	}
	for _, fn := range fns {
		fn(executor)
	}
	return executor
}

type ExecutorSetter func(*Executor)

type Executor struct {
	target     string
	uid        string
	port       int
	address    string
	meshClient *cli.MeshClient
	svcContext *cli.ServiceContext
}

func (e *Executor) Complete(options *Options) {
	// 与 options 进行对接必要的参数
	e.target = options.target
	e.uid = options.uid

	// 听取动态配置必要的参数
	e.address = options.address
}

func splitHostPort(address string) net.Listener {
	if _, _, err := net.SplitHostPort(address); err != nil {
		klog.Errorf("Invalid address: %s\n", address)
		panic(fmt.Sprintf("Invalid address: %s", address))
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		klog.Errorf("Failed to start config server: %v\n", err)
		panic(err)
	}
	return listener
}

func (e *Executor) routingServer(ctx context.Context) {
	// sidecar 的路由逻辑入口
}

func (e *Executor) Run(ctx context.Context) {
	meshClient := cli.NewMeshClient()
	meshClient.Connect(e.target)
	e.meshClient = meshClient

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		e.configServer(ctx)
	}()

	e.routingServer(ctx)
	wg.Wait()
}

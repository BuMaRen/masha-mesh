package app

import (
	"context"
	"sync"

	"github.com/BuMaRen/mesh/pkg/cli"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

func rootContext() context.Context {
	return context.Background()
}

func NewCommand() *cobra.Command {
	opts := &Options{}
	rootCmd := &cobra.Command{
		Use:   "mesh-cli",
		Short: "A brief description of your application",
		Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
		// Uncomment the following line if your bare application
		// has an action associated with it:
		Run: func(cmd *cobra.Command, args []string) {
			svcCache := cli.NewServiceCache(opts.cacheCapacity)
			meshClient := cli.NewMeshClient(opts.uid, svcCache)
			proxyServer, httpServer := opts.Complete(meshClient, cli.NewServiceContext())
			ctx := rootContext()
			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				// 启动l4/l7代理服务器
				// l4 监听 A 端口，l7 监听 B 端口
				// TODO：l7需要处理回流请求到app，回报直接写入net.Conn
				proxyServer.Run(ctx)
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				// 启动http服务器，用于l4/l7代理服务器所注册的服务
				// 走往注册服务的流量都会由l4/l7代理服务器接管
				if err := httpServer.Run(ctx); err != nil {
					klog.Errorf("http server run failed with error: %+v", err)
				}
			}()
			wg.Wait()
		},
	}
	opts.ParseFlags(rootCmd)
	return rootCmd
}

package app

import (
	"github.com/BuMaRen/mesh/cmd/cli/app/httpserver"
	"github.com/BuMaRen/mesh/cmd/cli/app/proxy"
	"github.com/BuMaRen/mesh/pkg/cli"
	"github.com/spf13/cobra"
)

type Options struct {
	// MeshClient 相关配置
	target        string
	uid           string
	cacheCapacity int
	// L4Proxy 相关配置
	l7Port    int
	l4Address string
	// L7Proxy 相关配置
	l7Address string
	// 动态配置修改相关的配置
	httpAddress string
}

func (o *Options) Complete(meshClient *cli.MeshClient, svcContext *cli.ServiceContext) (*proxy.Proxy, *httpserver.HttpServer) {
	p := proxy.NewProxyOptions(
		proxy.WithL4address(o.l4Address),
		proxy.WithL7Address(o.l7Address),
		proxy.WithL7Port(o.l7Port),
	).Complete(meshClient)
	h := httpserver.NewHttpServer(httpserver.WithAddress(o.httpAddress))
	h.Complete(meshClient, svcContext)
	return p, h
}

func (o *Options) ParseFlags(cmd *cobra.Command) {
	// meshClient 相关配置
	cmd.Flags().StringVarP(&o.target, "target", "t", "mesh-ctrl:50051", "Target service in format namespace/name")
	cmd.Flags().StringVar(&o.uid, "uid", "mesh-cli", "Unique identifier for the client")
	cmd.Flags().IntVarP(&o.cacheCapacity, "cache-capacity", "c", 100, "Capacity of the service cache")

	// l4 proxy 和 l7 proxy 的地址配置，l4 转发 l7 业务到 l7 proxy
	cmd.Flags().IntVar(&o.l7Port, "l7-port", 8080, "Port for L7 proxy to listen on")
	cmd.Flags().StringVar(&o.l4Address, "l4-address", ":15001", "Address for L4 proxy to listen on")

	// l7 proxy 的地址配置，默认为 :8080，可以通过命令行参数覆盖
	cmd.Flags().StringVar(&o.l7Address, "l7-address", ":8080", "Address for L7proxy to listen on")

	// 动态配置 http server 的地址
	cmd.Flags().StringVarP(&o.httpAddress, "dynamic-config-address", "d", ":9090", "Address for dynamic-config HTTP server to listen on")
}

package proxy

import (
	"github.com/BuMaRen/mesh/internal/cli/breaker"
	"github.com/BuMaRen/mesh/internal/cli/proxyconfig"
	"github.com/spf13/cobra"
)

type Options struct {
	listenAddress string
	configFile    string
	breakerOpts   *breaker.Options
	config        *proxyconfig.Config
}

func NewOptions() *Options {
	return &Options{
		listenAddress: ":8081",
		configFile:    "",
		breakerOpts:   breaker.NewOptions(),
	}
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.listenAddress, "listen-address", o.listenAddress, "Proxy listen address")
	cmd.Flags().StringVar(&o.configFile, "config", o.configFile, "Configuration file path")
	o.breakerOpts.AddFlags(cmd)
}

func (o *Options) ListenAddress() string {
	return o.listenAddress
}

func (o *Options) ConfigFile() string {
	return o.configFile
}

func (o *Options) BreakerOptions() *breaker.Options {
	return o.breakerOpts
}

func (o *Options) Config() *proxyconfig.Config {
	return o.config
}

func (o *Options) LoadConfig() error {
	if o.configFile == "" {
		o.config = &proxyconfig.Config{}
		return nil
	}
	config, err := proxyconfig.LoadConfig(o.configFile)
	if err != nil {
		return err
	}
	o.config = config
	return nil
}

package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

func WithSignalCatch(root context.Context) context.Context {
	ctx, cancel := context.WithCancel(root)
	ch := make(chan os.Signal, 1)
	signal.Reset(syscall.SIGTERM, syscall.SIGINT)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		// Listen for OS signals and call cancel() when received
		<-ch
		klog.Infof("Received shutdown signal, initiating graceful shutdown...")
		cancel()
		<-ch
		klog.Infof("Received second shutdown signal, forcing exit...")
		os.Exit(1)
	}()
	return ctx
}

func NewCommand() *cobra.Command {
	opts := NewOptions()
	rootCmd := &cobra.Command{
		Use:   "webhook",
		Short: "A brief description of your application",
		Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
		// Uncomment the following line if your bare application
		// has an action associated with it:
		RunE: func(cmd *cobra.Command, args []string) error {
			rootContext := WithSignalCatch(context.Background())
			return RunError(rootContext, opts)
		},
	}
	opts.AddFlags(rootCmd)
	return rootCmd
}

package app

import (
	"context"

	"github.com/BuMaRen/mesh/internal/ctrl"
	"github.com/spf13/cobra"
)

func NewStartUpCommand() *cobra.Command {
	startupOpts := ctrl.NewStartUpOptions()
	cmd := &cobra.Command{
		Use:   "startup",
		Short: "Start the mesh controller",
		Long:  `Start the mesh controller gracefully.`,
		Run: func(cmd *cobra.Command, args []string) {
			rootContext := WithSignalCatch(context.Background())
			workingContext, cancel := context.WithCancel(rootContext)
			defer cancel()
			ctrl.StartUp(workingContext, startupOpts)
		},
	}
	// 子命令需要自己注册 flags（父命令的普通 flags 不会被子命令继承）
	startupOpts.AddFlags(cmd)
	return cmd
}

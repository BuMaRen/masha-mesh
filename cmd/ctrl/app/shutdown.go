package app

import (
	"github.com/BuMaRen/mesh/internal/ctrl"
	"github.com/spf13/cobra"
)

func NewShutDownCommand(opt *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shutdown",
		Short: "Shutdown the mesh controller",
		Long:  `Shutdown the mesh controller gracefully.`,
		Run: func(cmd *cobra.Command, args []string) {
			// 这里可以添加一些清理资源的逻辑，例如关闭数据库连接、清理缓存等
			// 由于 StartUp 函数已经实现了 graceful shutdown，这里不需要额外的逻辑
			shutdown := ctrl.NewShutdown(opt.CtrlOptions())
			shutdown.Execute()
		},
	}
	// 子命令需要自己注册 flags（父命令的普通 flags 不会被子命令继承）
	opt.CtrlOptions().AddFlags(cmd)
	return cmd
}

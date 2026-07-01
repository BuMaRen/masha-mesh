package app

import (
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "mesh-ctrl",
		Short: "Mesh Controller is a component of the mesh system responsible for managing and controlling the mesh network.",
		Long: `Mesh Controller is a component of the mesh system responsible for managing and controlling the mesh network.
It provides functionalities to monitor, configure, and maintain the mesh network efficiently.`,
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help() // Show help if no subcommand is provided
		},
	}
	rootCmd.AddCommand(NewStartUpCommand())
	rootCmd.AddCommand(NewShutDownCommand())
	return rootCmd
}

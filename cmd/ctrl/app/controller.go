package app

import (
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	opts := NewOptions()
	rootCmd := &cobra.Command{
		Use:   "mesh-ctrl",
		Short: "Mesh Controller is a component of the mesh system responsible for managing and controlling the mesh network.",
		Long: `Mesh Controller is a component of the mesh system responsible for managing and controlling the mesh network.
It provides functionalities to monitor, configure, and maintain the mesh network efficiently.`,
		// Uncomment the following line if your bare application
		// has an action associated with it:
		Run: func(cmd *cobra.Command, args []string) {
			opts.Run()
		},
	}
	opts.AddFlags(rootCmd)
	rootCmd.AddCommand(NewShutDownCommand(opts))
	return rootCmd
}

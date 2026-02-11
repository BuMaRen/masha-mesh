package app

import (
	"os"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	opts := NewOptions()
	rootCmd := &cobra.Command{
		Use:   "mesh-ctrl",
		Short: "A brief description of your application",
		Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
		// Uncomment the following line if your bare application
		// has an action associated with it:
		Run: func(cmd *cobra.Command, args []string) {
			opts.Run()
		},
	}
	rootCmd.Flags().StringVarP(&opts.Namespace, "namespace", "n", "default", "Namespace to use")
	rootCmd.Flags().IntVarP(&opts.Port, "port", "p", 50051, "Port to listen on")
	opts.PodName = os.Getenv("POD_NAME")
	return rootCmd
}

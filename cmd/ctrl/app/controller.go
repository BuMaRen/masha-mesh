package app

import (
	"github.com/BuMaRen/mesh/pkg/controller"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	nameSpace := "default"
	port := 50051
	rootCmd := &cobra.Command{
		Use:   "mesh",
		Short: "A brief description of your application",
		Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
		// Uncomment the following line if your bare application
		// has an action associated with it:
		RunE: func(cmd *cobra.Command, args []string) error {
			ctl := controller.NewController(nameSpace)
			return ctl.RunAndServe(port)
		},
	}
	rootCmd.Flags().StringVarP(&nameSpace, "namespace", "n", "default", "Namespace to use")
	rootCmd.Flags().IntVarP(&port, "port", "p", 50051, "Port to listen on")
	return rootCmd
}

// // Execute adds all child commands to the root command and sets flags appropriately.
// // This is called by main.main(). It only needs to happen once to the rootCmd.
// func Execute() {
// 	err := rootCmd.Execute()
// 	if err != nil {
// 		os.Exit(1)
// 	}
// }

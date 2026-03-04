package main

import (
	"github.com/BuMaRen/mesh/cmd/cli/app"
)

func main() {
	command := app.NewCommand()
	if err := command.Execute(); err != nil {
		panic(err)
	}
}

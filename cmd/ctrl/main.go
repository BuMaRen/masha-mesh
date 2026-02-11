package main

import "github.com/BuMaRen/mesh/cmd/ctrl/app"

func main() {
	command := app.NewCommand()
	if err := command.Execute(); err != nil {
		panic(err)
	}
}

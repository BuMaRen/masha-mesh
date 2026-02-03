package app

import "github.com/BuMaRen/mesh/pkg/ctrl"

type Options struct {
	Namespace string
	Port      int
}

func NewOptions() *Options {
	return &Options{
		Namespace: "default",
		Port:      50051,
	}
}

func (o *Options) Run() {
	ctrl := ctrl.NewLogic(o.Namespace, o.Port)
	ctrl.Run()
}

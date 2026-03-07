package app

type Options struct {
	target  string
	uid     string
	svcName string

	address string
}

func NewOptions() *Options {
	return &Options{
		target:  "mesh-ctrl:50051",
		uid:     "mesh-sidecar",
		svcName: "mesh-ctrl",
		address: ":9090",
	}
}

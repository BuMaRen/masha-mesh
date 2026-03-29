package logic

type OptionSetter func(*Options)

type Options struct {
	// grpc 服务器的配置
	GrpcPort       int
	MapInitialSize int

	// https 服务器的配置
	Crt     string
	Key     string
	Address string
}

func WithGrpcPort(port int) OptionSetter {
	return func(o *Options) {
		o.GrpcPort = port
	}
}

func WithGrpcMapInitialSize(size int) OptionSetter {
	return func(o *Options) {
		o.MapInitialSize = size
	}
}

func WithHttpsCrtKey(crt, key string) OptionSetter {
	return func(o *Options) {
		o.Crt = crt
		o.Key = key
	}
}

func NewOptions(setters ...OptionSetter) *Options {
	opts := &Options{}
	for _, setter := range setters {
		setter(opts)
	}
	return opts
}

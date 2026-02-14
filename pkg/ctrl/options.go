package ctrl

import "google.golang.org/grpc"

type OptionFunc func(*Logic)

func WithGrpcPort(port int) OptionFunc {
	return func(l *Logic) {
		l.grpcPort = port
	}
}

func WithStorage(storage Storage) OptionFunc {
	return func(l *Logic) {
		l.core = storage
	}
}

func WithGrpcServer(grpcServer *grpc.Server) OptionFunc {
	return func(l *Logic) {
		l.compeletedGrpcServer = grpcServer
	}
}

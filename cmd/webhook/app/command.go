package app

import "context"

func RunError(ctx context.Context, opts *Options) error {
	newServer := NewHttpsServer()
	newServer.Complete(opts)
	return newServer.Serve(ctx)
}

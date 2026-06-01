package l7

import (
	"context"
	"net/http"

	"github.com/BuMaRen/mesh/internal/cli/rpcclient"
)

type Server struct {
	client *rpcclient.MeshClient
	// breakers *breaker.Breakers
}

func NewServer(meshClient *rpcclient.MeshClient) *Server {
	return &Server{
		client: meshClient,
		// breakers: breaker.NewBreakers(),
	}
}

func (s *Server) Run(ctx context.Context, opts *Options) error {
	handler := http.HandlerFunc(s.handleRequest)
	// s.breakers.Start(ctx, opts.breakerOpts)
	return http.ListenAndServe(opts.l7Address, handler)
}

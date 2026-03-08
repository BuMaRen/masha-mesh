package routeserver

import (
	"fmt"
	"net"

	"github.com/BuMaRen/mesh/pkg/cli"
	"github.com/gin-gonic/gin"
)

type OptionsFunc func(*RouteServer)

// type L4OptionsFunc func(*RouteServer)

type RouteServer struct {
	l4ip       string
	l4port     int
	l7ip       string
	l7port     int
	meshClient *cli.MeshClient
	l7engine   *gin.Engine
}

func NewRouteServer(meshClient *cli.MeshClient, opts ...OptionsFunc) *RouteServer {
	rs := &RouteServer{
		meshClient: meshClient,
	}
	for _, opt := range opts {
		opt(rs)
	}
	return rs
}

func (s *RouteServer) Complete() error {
	if net.ParseIP(s.l4ip) == nil {
		return fmt.Errorf("invalid L4 IP: %s", s.l4ip)
	}
	if net.ParseIP(s.l7ip) == nil {
		return fmt.Errorf("invalid L7 IP: %s", s.l7ip)
	}
	s.l7engine = gin.Default()
	return nil
}

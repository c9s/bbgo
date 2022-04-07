package grpc

import (
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/pb"
)

import "google.golang.org/grpc"


type Server struct {
	Config        *bbgo.Config
	Environ       *bbgo.Environment
	Trader        *bbgo.Trader
}

func (s *Server) Run(bind string) error {
	var grpcServer = grpc.NewServer()
	_ = grpcServer
	_ = pb.QueryTradesResponse{}
	return nil
}

package grpc

import (
	"context"

	"google.golang.org/grpc"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/pb"
	"github.com/c9s/bbgo/pkg/types"
)

type Server struct {
	Config  *bbgo.Config
	Environ *bbgo.Environment
	Trader  *bbgo.Trader

	pb.UnimplementedMarketDataServiceServer
}

func (s *Server) Subscribe(request *pb.SubscribeRequest, server pb.MarketDataService_SubscribeServer) error {
	panic("implement me")
	return nil
}

func (s *Server) QueryKLines(ctx context.Context, request *pb.QueryKLinesRequest) (*pb.QueryKLinesResponse, error) {
	exchangeName, err := types.ValidExchangeName(request.Exchange)
	if err != nil {
		return nil, err
	}

	for _, session := range s.Environ.Sessions() {
		if session.ExchangeName == exchangeName {
			options := types.KLineQueryOptions{
				Limit: int(request.Limit),
			}

			klines, err := session.Exchange.QueryKLines(ctx, request.Symbol, types.Interval(request.Interval), options)
			if err != nil {
				return nil, err
			}
			_ = klines

			return nil, nil
		}
	}

	return nil, nil
}

func (s *Server) Run(bind string) error {
	var grpcServer = grpc.NewServer()
	pb.RegisterMarketDataServiceServer(grpcServer, s)
	return nil
}

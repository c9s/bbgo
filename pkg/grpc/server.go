package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/pb"
	"github.com/c9s/bbgo/pkg/types"
)

type Server struct {
	Config  *bbgo.Config
	Environ *bbgo.Environment
	Trader  *bbgo.Trader

	pb.UnimplementedMarketDataServiceServer
	pb.UnimplementedUserDataServiceServer
}

func (s *Server) SubscribeUserData(empty *pb.Empty, server pb.UserDataService_SubscribeServer) error {
	return nil
}

func (s *Server) Subscribe(request *pb.SubscribeRequest, server pb.MarketDataService_SubscribeServer) error {
	exchangeSubscriptions := map[string][]types.Subscription{}
	for _, sub := range request.Subscriptions {
		session, ok := s.Environ.Session(sub.Exchange)
		if !ok {
			return fmt.Errorf("exchange %s not found", sub.Exchange)
		}

		ss, err := toSubscriptions(sub)
		if err != nil {
			return err
		}

		exchangeSubscriptions[session.Name] = append(exchangeSubscriptions[session.Name], ss)
	}

	streamPool := map[string]types.Stream{}
	for sessionName, subs := range exchangeSubscriptions {
		session, ok := s.Environ.Session(sessionName)
		if !ok {
			log.Errorf("session %s not found", sessionName)
			continue
		}

		stream := session.Exchange.NewStream()
		stream.SetPublicOnly()
		for _, sub := range subs {
			log.Infof("%s subscribe %s %s %+v", sessionName, sub.Channel, sub.Symbol, sub.Options)
			stream.Subscribe(sub.Channel, sub.Symbol, sub.Options)
		}

		stream.OnMarketTrade(func(trade types.Trade) {
			if err := server.Send(transMarketTrade(session, trade)); err != nil {
				log.WithError(err).Error("grpc stream send error")
			}
		})

		stream.OnBookSnapshot(func(book types.SliceOrderBook) {
			if err := server.Send(transBook(session, book, pb.Event_SNAPSHOT)); err != nil {
				log.WithError(err).Error("grpc stream send error")
			}
		})

		stream.OnBookUpdate(func(book types.SliceOrderBook) {
			if err := server.Send(transBook(session, book, pb.Event_UPDATE)); err != nil {
				log.WithError(err).Error("grpc stream send error")
			}
		})
		stream.OnKLineClosed(func(kline types.KLine) {
			err := server.Send(transKLineResponse(session, kline))
			if err != nil {
				log.WithError(err).Error("grpc stream send error")
			}
		})
		streamPool[sessionName] = stream
	}

	for _, stream := range streamPool {
		go stream.Connect(server.Context())
	}

	defer func() {
		for _, stream := range streamPool {
			if err := stream.Close(); err != nil {
				log.WithError(err).Errorf("stream close error")
			}
		}
	}()

	ctx := server.Context()
	<-ctx.Done()
	return ctx.Err()
}

func (s *Server) QueryKLines(ctx context.Context, request *pb.QueryKLinesRequest) (*pb.QueryKLinesResponse, error) {
	exchangeName, err := types.ValidExchangeName(request.Exchange)
	if err != nil {
		return nil, err
	}

	for _, session := range s.Environ.Sessions() {
		if session.ExchangeName == exchangeName {
			response := &pb.QueryKLinesResponse{
				Klines: nil,
				Error:  nil,
			}

			options := types.KLineQueryOptions{
				Limit: int(request.Limit),
			}

			endTime := time.Now()
			if request.EndTime != 0 {
				endTime = time.Unix(request.EndTime, 0)
			}
			options.EndTime = &endTime

			if request.StartTime != 0 {
				startTime := time.Unix(request.StartTime, 0)
				options.StartTime = &startTime
			}

			klines, err := session.Exchange.QueryKLines(ctx, request.Symbol, types.Interval(request.Interval), options)
			if err != nil {
				return nil, err
			}

			for _, kline := range klines {
				response.Klines = append(response.Klines, transKLine(session, kline))
			}

			return response, nil
		}
	}

	return nil, nil
}

func (s *Server) ListenAndServe(bind string) error {
	conn, err := net.Listen("tcp", bind)
	if err != nil {
		return errors.Wrapf(err, "failed to bind network at %s", bind)
	}

	var grpcServer = grpc.NewServer()
	pb.RegisterMarketDataServiceServer(grpcServer, s)

	reflection.Register(grpcServer)

	if err := grpcServer.Serve(conn); err != nil {
		return errors.Wrap(err, "failed to serve grpc connections")
	}

	return nil
}


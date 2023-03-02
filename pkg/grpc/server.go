package grpc

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/pb"
	"github.com/c9s/bbgo/pkg/types"
)

type TradingService struct {
	Config  *bbgo.Config
	Environ *bbgo.Environment
	Trader  *bbgo.Trader

	pb.UnimplementedTradingServiceServer
}

func (s *TradingService) SubmitOrder(ctx context.Context, request *pb.SubmitOrderRequest) (*pb.SubmitOrderResponse, error) {
	sessionName := request.Session

	if len(sessionName) == 0 {
		return nil, fmt.Errorf("session name can not be empty")
	}

	session, ok := s.Environ.Session(sessionName)
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionName)
	}

	submitOrders := toSubmitOrders(request.SubmitOrders)
	for i := range submitOrders {
		if market, ok := session.Market(submitOrders[i].Symbol); ok {
			submitOrders[i].Market = market
		} else {
			log.Warnf("session %s market %s not found", sessionName, submitOrders[i].Symbol)
		}
	}

	// we will return this error later because some orders could be succeeded
	createdOrders, _, err := bbgo.BatchRetryPlaceOrder(ctx, session.Exchange, nil, nil, log.StandardLogger(), submitOrders...)

	// convert response
	resp := &pb.SubmitOrderResponse{
		Session: sessionName,
		Orders:  nil,
	}

	for _, createdOrder := range createdOrders {
		resp.Orders = append(resp.Orders, transOrder(session, createdOrder))
	}

	return resp, err
}

func (s *TradingService) CancelOrder(ctx context.Context, request *pb.CancelOrderRequest) (*pb.CancelOrderResponse, error) {
	sessionName := request.Session

	if len(sessionName) == 0 {
		return nil, fmt.Errorf("session name can not be empty")
	}

	session, ok := s.Environ.Session(sessionName)
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionName)
	}

	uuidOrderID := ""
	orderID, err := strconv.ParseUint(request.OrderId, 10, 64)
	if err != nil {
		// TODO: validate uuid
		uuidOrderID = request.OrderId
	}

	session.Exchange.CancelOrders(ctx, types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: request.ClientOrderId,
		},
		OrderID: orderID,
		UUID:    uuidOrderID,
	})

	resp := &pb.CancelOrderResponse{}
	return resp, nil
}

func (s *TradingService) QueryOrder(ctx context.Context, request *pb.QueryOrderRequest) (*pb.QueryOrderResponse, error) {
	panic("implement me")
}

func (s *TradingService) QueryOrders(ctx context.Context, request *pb.QueryOrdersRequest) (*pb.QueryOrdersResponse, error) {
	panic("implement me")
}

func (s *TradingService) QueryTrades(ctx context.Context, request *pb.QueryTradesRequest) (*pb.QueryTradesResponse, error) {
	panic("implement me")
}

type UserDataService struct {
	Config  *bbgo.Config
	Environ *bbgo.Environment
	Trader  *bbgo.Trader

	pb.UnimplementedUserDataServiceServer
}

func (s *UserDataService) Subscribe(request *pb.UserDataRequest, server pb.UserDataService_SubscribeServer) error {
	sessionName := request.Session

	if len(sessionName) == 0 {
		return fmt.Errorf("session name can not be empty")
	}

	session, ok := s.Environ.Session(sessionName)
	if !ok {
		return fmt.Errorf("session %s not found", sessionName)
	}

	userDataStream := session.Exchange.NewStream()
	userDataStream.OnOrderUpdate(func(order types.Order) {
		err := server.Send(&pb.UserData{
			Channel: pb.Channel_ORDER,
			Event:   pb.Event_UPDATE,
			Orders:  []*pb.Order{transOrder(session, order)},
		})
		if err != nil {
			log.WithError(err).Errorf("grpc: can not send user data")
		}
	})
	userDataStream.OnTradeUpdate(func(trade types.Trade) {
		err := server.Send(&pb.UserData{
			Channel: pb.Channel_TRADE,
			Event:   pb.Event_UPDATE,
			Trades:  []*pb.Trade{transTrade(session, trade)},
		})
		if err != nil {
			log.WithError(err).Errorf("grpc: can not send user data")
		}
	})

	balanceHandler := func(balances types.BalanceMap) {
		err := server.Send(&pb.UserData{
			Channel:  pb.Channel_BALANCE,
			Event:    pb.Event_UPDATE,
			Balances: transBalances(session, balances),
		})
		if err != nil {
			log.WithError(err).Errorf("grpc: can not send user data")
		}
	}
	userDataStream.OnBalanceUpdate(balanceHandler)
	userDataStream.OnBalanceSnapshot(balanceHandler)

	ctx := server.Context()

	balances, err := session.Exchange.QueryAccountBalances(ctx)
	if err != nil {
		return err
	}

	err = server.Send(&pb.UserData{
		Channel:  pb.Channel_BALANCE,
		Event:    pb.Event_SNAPSHOT,
		Balances: transBalances(session, balances),
	})
	if err != nil {
		log.WithError(err).Errorf("grpc: can not send user data")
	}

	go userDataStream.Connect(ctx)

	defer func() {
		if err := userDataStream.Close(); err != nil {
			log.WithError(err).Errorf("user data stream close error")
		}
	}()

	<-ctx.Done()
	return nil
}

type MarketDataService struct {
	Config  *bbgo.Config
	Environ *bbgo.Environment
	Trader  *bbgo.Trader

	pb.UnimplementedMarketDataServiceServer
}

func (s *MarketDataService) Subscribe(request *pb.SubscribeRequest, server pb.MarketDataService_SubscribeServer) error {
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
				log.WithError(err).Errorf("market data stream close error")
			}
		}
	}()

	ctx := server.Context()
	<-ctx.Done()
	return ctx.Err()
}

func (s *MarketDataService) QueryKLines(ctx context.Context, request *pb.QueryKLinesRequest) (*pb.QueryKLinesResponse, error) {
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

type Server struct {
	Config  *bbgo.Config
	Environ *bbgo.Environment
	Trader  *bbgo.Trader
}

func (s *Server) ListenAndServe(bind string) error {
	conn, err := net.Listen("tcp", bind)
	if err != nil {
		return errors.Wrapf(err, "failed to bind network at %s", bind)
	}

	var grpcServer = grpc.NewServer()
	pb.RegisterMarketDataServiceServer(grpcServer, &MarketDataService{
		Config:  s.Config,
		Environ: s.Environ,
		Trader:  s.Trader,
	})

	pb.RegisterTradingServiceServer(grpcServer, &TradingService{
		Config:  s.Config,
		Environ: s.Environ,
		Trader:  s.Trader,
	})

	pb.RegisterUserDataServiceServer(grpcServer, &UserDataService{
		Config:  s.Config,
		Environ: s.Environ,
		Trader:  s.Trader,
	})

	reflection.Register(grpcServer)

	if err := grpcServer.Serve(conn); err != nil {
		return errors.Wrap(err, "failed to serve grpc connections")
	}

	return nil
}

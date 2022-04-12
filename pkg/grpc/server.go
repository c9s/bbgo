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
}

func (s *Server) Subscribe(request *pb.SubscribeRequest, server pb.MarketDataService_SubscribeServer) error {
	exchangeSubscriptions := map[string][]types.Subscription{}
	for _, sub := range request.Subscriptions {
		session, ok := s.Environ.Session(sub.Exchange)
		if !ok {
			return fmt.Errorf("exchange %s not found", sub.Exchange)
		}

		switch sub.Channel {
		case pb.Channel_TRADE:
			exchangeSubscriptions[session.Name] = append(exchangeSubscriptions[session.Name], types.Subscription{
				Symbol:  sub.Symbol,
				Channel: types.MarketTradeChannel,
			})

		case pb.Channel_BOOK:
			exchangeSubscriptions[session.Name] = append(exchangeSubscriptions[session.Name], types.Subscription{
				Symbol:  sub.Symbol,
				Channel: types.BookChannel,
				Options: types.SubscribeOptions{
					Depth: types.Depth(sub.Depth),
				},
			})

		case pb.Channel_KLINE:
			exchangeSubscriptions[session.Name] = append(exchangeSubscriptions[session.Name], types.Subscription{
				Symbol:  sub.Symbol,
				Channel: types.KLineChannel,
				Options: types.SubscribeOptions{
					Interval: sub.Interval,
				},
			})
		}
	}

	streamPool := map[string]types.Stream{}
	for sessionName, subs := range exchangeSubscriptions {
		if session, ok := s.Environ.Session(sessionName); ok {
			stream := session.Exchange.NewStream()
			stream.SetPublicOnly()
			for _, sub := range subs {
				stream.Subscribe(sub.Channel, sub.Symbol, sub.Options)
			}

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
				err := server.Send(transKLine(session, kline))
				if err != nil {
					log.WithError(err).Error("grpc stream send error")
				}
			})
			streamPool[sessionName] = stream
		}
	}

	for sessionName, stream := range streamPool {
		log.Infof("connecting stream %s", sessionName)
		go stream.Connect(server.Context())
	}

	return nil
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
				response.Klines = append(response.Klines, &pb.KLine{
					Exchange:    kline.Exchange.String(),
					Symbol:      kline.Symbol,
					StartTime:   kline.StartTime.UnixMilli(),
					Open:        kline.Open.Float64(),
					High:        kline.High.Float64(),
					Low:         kline.Low.Float64(),
					Close:       kline.Close.Float64(),
					Volume:      kline.Volume.Float64(),
					QuoteVolume: kline.QuoteVolume.Float64(),
				})
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

func transPriceVolume(srcPvs types.PriceVolumeSlice) (pvs []*pb.PriceVolume) {
	for _, srcPv := range srcPvs {
		pvs = append(pvs, &pb.PriceVolume{
			Price:  srcPv.Price.String(),
			Volume: srcPv.Volume.String(),
		})
	}
	return pvs
}

func transBook(session *bbgo.ExchangeSession, book types.SliceOrderBook, event pb.Event) *pb.SubscribeResponse {
	return &pb.SubscribeResponse{
		Session:  session.Name,
		Exchange: session.ExchangeName.String(),
		Symbol:   book.Symbol,
		Channel:  pb.Channel_BOOK,
		Event:    event, // pb.Event_UPDATE
		Depth: &pb.Depth{
			Exchange: session.ExchangeName.String(),
			Symbol:   book.Symbol,
			Asks:     transPriceVolume(book.Asks),
			Bids:     transPriceVolume(book.Bids),
		},
	}
}

func transKLine(session *bbgo.ExchangeSession, kline types.KLine) *pb.SubscribeResponse {
	return &pb.SubscribeResponse{
		Session:  session.Name,
		Exchange: kline.Exchange.String(),
		Symbol:   kline.Symbol,
		Channel:  pb.Channel_KLINE,
		Event:    pb.Event_UPDATE,
		Kline: &pb.KLine{
			Session:     session.Name,
			Exchange:    kline.Exchange.String(),
			Symbol:      kline.Symbol,
			Open:        kline.Open.Float64(),
			High:        kline.High.Float64(),
			Low:         kline.Low.Float64(),
			Close:       kline.Close.Float64(),
			Volume:      kline.Volume.Float64(),
			QuoteVolume: kline.QuoteVolume.Float64(),
			StartTime:   kline.StartTime.UnixMilli(),
			EndTime:     kline.StartTime.UnixMilli(),
		},
		SubscribedAt: 0,
	}
}

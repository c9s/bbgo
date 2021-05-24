package ftx

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

const endpoint = "wss://ftx.com/ws/"

type Stream struct {
	*types.StandardStream

	ws           *service.WebsocketClientBase
	klineMessage chan types.KLine
	exchange     *Exchange
	ctx          context.Context
	isConnected  bool

	// publicOnly can only be configured before connecting
	publicOnly int32

	key        string
	secret     string
	subAccount string

	// subscriptions are only accessed in single goroutine environment, so I don't use mutex to protect them
	subscriptions []websocketRequest
}

func NewStream(key, secret string, subAccount string, e *Exchange) *Stream {
	s := &Stream{
		exchange:       e,
		isConnected:    false,
		key:            key,
		klineMessage:   make(chan types.KLine),
		secret:         secret,
		subAccount:     subAccount,
		StandardStream: &types.StandardStream{},
		ws:             service.NewWebsocketClientBase(endpoint, 3*time.Second),
	}

	s.ws.OnMessage((&messageHandler{StandardStream: s.StandardStream}).handleMessage)
	s.ws.OnConnected(func(conn *websocket.Conn) {
		subs := []websocketRequest{newLoginRequest(s.key, s.secret, time.Now(), s.subAccount)}
		subs = append(subs, s.subscriptions...)
		for _, sub := range subs {
			if err := conn.WriteJSON(sub); err != nil {
				s.ws.EmitError(fmt.Errorf("failed to send subscription: %+v", sub))
			}
		}

		s.EmitConnect()
	})

	return s
}

func (s *Stream) Connect(ctx context.Context) error {
	// If it's not public only, let's do the authentication.
	if atomic.LoadInt32(&s.publicOnly) == 0 {
		s.subscribePrivateEvents()
	}

	if err := s.ws.Connect(ctx); err != nil {
		return err
	}
	s.ctx = ctx
	s.isConnected = true
	s.EmitStart()
	go s.handleChannelKlineMessage()

	go func() {
		// https://docs.ftx.com/?javascript#request-process
		tk := time.NewTicker(15 * time.Second)
		defer tk.Stop()
		for {
			select {
			case <-ctx.Done():
				if err := ctx.Err(); err != nil {
					logger.WithError(err).Errorf("websocket ping goroutine is terminated")
				}
			case <-tk.C:
				if err := s.ws.Conn().WriteJSON(websocketRequest{
					Operation: ping,
				}); err != nil {
					logger.WithError(err).Warnf("failed to ping, try in next tick")
				}
			}
		}
	}()
	return nil
}

func (s *Stream) subscribePrivateEvents() {
	s.addSubscription(websocketRequest{
		Operation: subscribe,
		Channel:   privateOrdersChannel,
	})
	s.addSubscription(websocketRequest{
		Operation: subscribe,
		Channel:   privateTradesChannel,
	})
}

func (s *Stream) addSubscription(request websocketRequest) {
	s.subscriptions = append(s.subscriptions, request)
}

func (s *Stream) SetPublicOnly() {
	atomic.StoreInt32(&s.publicOnly, 1)
}

func (s *Stream) Subscribe(channel types.Channel, symbol string, option types.SubscribeOptions) {
	if channel == types.BookChannel {
		s.addSubscription(websocketRequest{
			Operation: subscribe,
			Channel:   orderBookChannel,
			Market:    toLocalSymbol(TrimUpperString(symbol)),
		})

	} else if channel == types.KLineChannel {
		// FTX does not support kline channel, do polling
		go s.subscribeKLine(symbol, option)
	} else {
		panic("only support book/kline channel now")
	}
}

func (s *Stream) handleChannelKlineMessage() {
	for {
		kline := <-s.klineMessage

		if kline.Closed {
			s.EmitKLineClosed(kline)
			continue
		}

		s.EmitKLine(kline)
	}
}

func (s *Stream) subscribeKLine(symbol string, option types.SubscribeOptions) {
	interval := types.Interval(option.Interval)
	if !isIntervalSupportedInKLine(interval) {
		logger.Errorf("not supported kline interval %s", option.Interval)
		return
	}

	ticker := time.NewTicker(interval.Duration())
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			if err := s.ctx.Err(); err != nil {
				logger.WithError(err).Errorf("subscribeKLine goroutine is terminated")
			}
		case <-ticker.C:
			klines := getLastKLine(s.exchange, s.ctx, symbol, interval)
			if len(klines) >= 0 {
				// handle mutiple klines, get the latest one
				s.klineMessage <- klines[len(klines)-1]
			}
		}
	}
}

func getLastKLine(e *Exchange, ctx context.Context, symbol string, interval types.Interval) []types.KLine {
	// set since to more 30s ago to avoid getting no kline candle
	since := time.Now().Add(time.Duration(-1*(interval.Minutes()*60+30)) * time.Second)
	klines, err := e.QueryKLines(ctx, symbol, interval, types.KLineQueryOptions{
		StartTime: &since,
	})
	if err != nil {
		logger.WithError(err).Errorf("failed to get kline data")
		return klines
	}

	return klines
}

func (s *Stream) Close() error {
	s.subscriptions = nil
	if s.ws != nil {
		return s.ws.Conn().Close()
	}
	return nil
}

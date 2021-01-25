package binance

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/gorilla/websocket"

	"github.com/c9s/bbgo/pkg/fixedpoint"

	"github.com/c9s/bbgo/pkg/types"
)

var debugBinanceDepth bool

func init() {
	// randomize pulling
	rand.Seed(time.Now().UnixNano())

	if s := os.Getenv("BINANCE_DEBUG_DEPTH"); len(s) > 0 {
		v, err := strconv.ParseBool(s)
		if err != nil {
			log.Error(err)
		} else {
			debugBinanceDepth = v
			if debugBinanceDepth {
				log.Info("binance depth debugging is enabled")
			}
		}
	}
}

type StreamRequest struct {
	// request ID is required
	ID     int      `json:"id"`
	Method string   `json:"method"`
	Params []string `json:"params"`
}

//go:generate callbackgen -type Stream -interface
type Stream struct {
	types.MarginSettings

	types.StandardStream

	Client    *binance.Client
	ListenKey string
	Conn      *websocket.Conn
	connLock  sync.Mutex

	publicOnly bool

	// custom callbacks
	depthEventCallbacks       []func(e *DepthEvent)
	kLineEventCallbacks       []func(e *KLineEvent)
	kLineClosedEventCallbacks []func(e *KLineEvent)

	balanceUpdateEventCallbacks           []func(event *BalanceUpdateEvent)
	outboundAccountInfoEventCallbacks     []func(event *OutboundAccountInfoEvent)
	outboundAccountPositionEventCallbacks []func(event *OutboundAccountPositionEvent)
	executionReportEventCallbacks         []func(event *ExecutionReportEvent)

	depthFrames map[string]*DepthFrame
}

func NewStream(client *binance.Client) *Stream {
	stream := &Stream{
		Client:      client,
		depthFrames: make(map[string]*DepthFrame),
	}

	stream.OnDepthEvent(func(e *DepthEvent) {
		f, ok := stream.depthFrames[e.Symbol]
		if !ok {
			f = &DepthFrame{
				client:  client,
				context: context.Background(),
				Symbol:  e.Symbol,
			}

			stream.depthFrames[e.Symbol] = f

			f.OnReady(func(e DepthEvent, bufEvents []DepthEvent) {
				snapshot, err := e.OrderBook()
				if err != nil {
					log.WithError(err).Error("book snapshot convert error")
					return
				}

				if valid, err := snapshot.IsValid(); !valid {
					log.Warnf("depth snapshot is invalid, event: %+v, error: %v", e, err)
				}

				stream.EmitBookSnapshot(snapshot)

				for _, e := range bufEvents {
					book, err := e.OrderBook()
					if err != nil {
						log.WithError(err).Error("book convert error")
						return
					}

					stream.EmitBookUpdate(book)
				}
			})

			f.OnPush(func(e DepthEvent) {
				book, err := e.OrderBook()
				if err != nil {
					log.WithError(err).Error("book convert error")
					return
				}

				stream.EmitBookUpdate(book)
			})
		} else {
			f.PushEvent(*e)
		}
	})

	stream.OnOutboundAccountPositionEvent(func(e *OutboundAccountPositionEvent) {
		snapshot := types.BalanceMap{}
		for _, balance := range e.Balances {
			available := fixedpoint.Must(fixedpoint.NewFromString(balance.Free))
			locked := fixedpoint.Must(fixedpoint.NewFromString(balance.Locked))
			snapshot[balance.Asset] = types.Balance{
				Currency:  balance.Asset,
				Available: available,
				Locked:    locked,
			}
		}
		stream.EmitBalanceSnapshot(snapshot)
	})

	stream.OnKLineEvent(func(e *KLineEvent) {
		kline := e.KLine.KLine()
		if e.KLine.Closed {
			stream.EmitKLineClosedEvent(e)
			stream.EmitKLineClosed(kline)
		} else {
			stream.EmitKLine(kline)
		}
	})

	stream.OnExecutionReportEvent(func(e *ExecutionReportEvent) {
		switch e.CurrentExecutionType {

		case "NEW", "CANCELED", "REJECTED", "EXPIRED", "REPLACED":
			order, err := e.Order()
			if err != nil {
				log.WithError(err).Error("order convert error")
				return
			}

			stream.EmitOrderUpdate(*order)

		case "TRADE":
			trade, err := e.Trade()
			if err != nil {
				log.WithError(err).Error("trade convert error")
				return
			}

			stream.EmitTradeUpdate(*trade)
		}
	})

	stream.OnConnect(func() {
		// reset the previous frames
		for _, f := range stream.depthFrames {
			f.reset()
			f.loadDepthSnapshot()
		}

		var params []string
		for _, subscription := range stream.Subscriptions {
			params = append(params, convertSubscription(subscription))
		}

		if len(params) == 0 {
			return
		}

		log.Infof("subscribing channels: %+v", params)
		err := stream.Conn.WriteJSON(StreamRequest{
			Method: "SUBSCRIBE",
			Params: params,
			ID:     1,
		})

		if err != nil {
			log.WithError(err).Error("subscribe error")
		}
	})

	return stream
}

func (s *Stream) SetPublicOnly() {
	s.publicOnly = true
}

func (s *Stream) dial(listenKey string) (*websocket.Conn, error) {
	var url string
	if s.publicOnly {
		url = "wss://stream.binance.com:9443/ws"
	} else {
		url = "wss://stream.binance.com:9443/ws/" + listenKey
	}

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (s *Stream) fetchListenKey(ctx context.Context) (string, error) {
	if s.IsMargin {
		if s.IsIsolatedMargin {
			log.Infof("isolated margin %s is enabled, requesting margin user stream listen key...", s.IsolatedMarginSymbol)
			req := s.Client.NewStartIsolatedMarginUserStreamService()
			req.Symbol(s.IsolatedMarginSymbol)
			return req.Do(ctx)
		}

		log.Infof("margin mode is enabled, requesting margin user stream listen key...")
		req := s.Client.NewStartMarginUserStreamService()
		return req.Do(ctx)
	}

	return s.Client.NewStartUserStreamService().Do(ctx)
}

func (s *Stream) keepaliveListenKey(ctx context.Context, listenKey string) error {
	if s.IsMargin {
		if s.IsIsolatedMargin {
			req := s.Client.NewKeepaliveIsolatedMarginUserStreamService().ListenKey(listenKey)
			req.Symbol(s.IsolatedMarginSymbol)
			return req.Do(ctx)
		}

		req := s.Client.NewKeepaliveMarginUserStreamService().ListenKey(listenKey)
		return req.Do(ctx)
	}

	return s.Client.NewKeepaliveUserStreamService().ListenKey(listenKey).Do(ctx)
}

func (s *Stream) connect(ctx context.Context) error {
	if s.publicOnly {
		log.Infof("stream is set to public only mode")
	} else {
		log.Infof("request listen key for creating user data stream...")

		listenKey, err := s.fetchListenKey(ctx)
		if err != nil {
			return err
		}

		s.ListenKey = listenKey
		log.Infof("user data stream created. listenKey: %s", maskListenKey(s.ListenKey))
	}

	conn, err := s.dial(s.ListenKey)
	if err != nil {
		return err
	}

	log.Infof("websocket connected")

	s.connLock.Lock()
	s.Conn = conn
	s.connLock.Unlock()

	s.EmitConnect()
	return nil
}

func convertSubscription(s types.Subscription) string {
	// binance uses lower case symbol name,
	// for kline, it's "<symbol>@kline_<interval>"
	// for depth, it's "<symbol>@depth OR <symbol>@depth@100ms"
	switch s.Channel {
	case types.KLineChannel:
		return fmt.Sprintf("%s@%s_%s", strings.ToLower(s.Symbol), s.Channel, s.Options.String())

	case types.BookChannel:
		return fmt.Sprintf("%s@depth", strings.ToLower(s.Symbol))
	}

	return fmt.Sprintf("%s@%s", strings.ToLower(s.Symbol), s.Channel)
}

func (s *Stream) Connect(ctx context.Context) error {
	err := s.connect(ctx)
	if err != nil {
		return err
	}

	go s.read(ctx)
	return nil
}

func (s *Stream) read(ctx context.Context) {

	pingTicker := time.NewTicker(10 * time.Second)
	defer pingTicker.Stop()

	keepAliveTicker := time.NewTicker(5 * time.Minute)
	defer keepAliveTicker.Stop()

	go func() {
		for {
			select {

			case <-ctx.Done():
				return

			case <-pingTicker.C:
				s.connLock.Lock()
				if err := s.Conn.WriteControl(websocket.PingMessage, []byte("hb"), time.Now().Add(3*time.Second)); err != nil {
					log.WithError(err).Error("ping error", err)
				}

				s.connLock.Unlock()

			case <-keepAliveTicker.C:
				if !s.publicOnly {
					if err := s.keepaliveListenKey(ctx, s.ListenKey); err != nil {
						log.WithError(err).Errorf("listen key keep-alive error: %v key: %s", err, maskListenKey(s.ListenKey))
					}
				}

			}
		}
	}()

	for {
		select {

		case <-ctx.Done():
			return

		default:
			if err := s.Conn.SetReadDeadline(time.Now().Add(1 * time.Minute)); err != nil {
				log.WithError(err).Errorf("set read deadline error: %s", err.Error())
			}

			mt, message, err := s.Conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
					log.WithError(err).Errorf("read error: %s", err.Error())
				} else {
					log.Info("websocket connection closed, going away")
				}

				// reconnect
				for err != nil {
					select {
					case <-ctx.Done():
						return

					default:
						if !s.publicOnly {
							if err := s.invalidateListenKey(ctx, s.ListenKey); err != nil {
								log.WithError(err).Error("invalidate listen key error")
							}
						}

						err = s.connect(ctx)
						time.Sleep(5 * time.Second)
					}
				}

				continue
			}

			// skip non-text messages
			if mt != websocket.TextMessage {
				continue
			}

			log.Debug(string(message))

			e, err := ParseEvent(string(message))
			if err != nil {
				log.WithError(err).Errorf("[binance] event parse error")
				continue
			}

			// log.NotifyTo("[binance] event: %+v", e)
			switch e := e.(type) {

			case *OutboundAccountPositionEvent:
				log.Info(e.Event, " ", e.Balances)
				s.EmitOutboundAccountPositionEvent(e)

			case *OutboundAccountInfoEvent:
				log.Info(e.Event, " ", e.Balances)
				s.EmitOutboundAccountInfoEvent(e)

			case *BalanceUpdateEvent:
				log.Info(e.Event, " ", e.Asset, " ", e.Delta)
				s.EmitBalanceUpdateEvent(e)

			case *KLineEvent:
				s.EmitKLineEvent(e)

			case *DepthEvent:
				s.EmitDepthEvent(e)

			case *ExecutionReportEvent:
				log.Info(e.Event, " ", e)
				s.EmitExecutionReportEvent(e)
			}
		}
	}
}

func (s *Stream) invalidateListenKey(ctx context.Context, listenKey string) (err error) {
	// should use background context to invalidate the user stream
	log.Info("closing listen key")

	if s.IsMargin {
		if s.IsIsolatedMargin {
			req := s.Client.NewCloseIsolatedMarginUserStreamService().ListenKey(listenKey)
			req.Symbol(s.IsolatedMarginSymbol)
			err = req.Do(ctx)
		} else {
			req := s.Client.NewCloseMarginUserStreamService().ListenKey(listenKey)
			err = req.Do(ctx)
		}

	} else {
		err = s.Client.NewCloseUserStreamService().ListenKey(listenKey).Do(ctx)
	}

	if err != nil {
		log.WithError(err).Error("error deleting listen key")
		return err
	}

	return nil
}

func (s *Stream) Close() error {
	log.Infof("closing user data stream...")

	if !s.publicOnly {
		if err := s.invalidateListenKey(context.Background(), s.ListenKey); err != nil {
			log.WithError(err).Error("invalidate listen key error")
		}
		log.Infof("user data stream closed")
	}

	s.connLock.Lock()
	defer s.connLock.Unlock()

	return s.Conn.Close()
}

func maskListenKey(listenKey string) string {
	maskKey := listenKey[0:5]
	return maskKey + strings.Repeat("*", len(listenKey)-1-5)
}

//go:generate callbackgen -type DepthFrame

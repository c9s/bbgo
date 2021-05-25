package binance

import (
	"context"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/gorilla/websocket"

	"github.com/c9s/bbgo/pkg/types"
)

var debugBinanceDepth bool

func init() {
	// randomize pulling
	rand.Seed(time.Now().UnixNano())

	debugBinanceDepth, _ = strconv.ParseBool(os.Getenv("DEBUG_BINANCE_DEPTH"))
	if debugBinanceDepth {
		log.Info("binance depth debugging is enabled")
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

	Client     *binance.Client
	ListenKey  string
	Conn       *websocket.Conn
	connLock   sync.Mutex
	reconnectC chan struct{}

	connCtx    context.Context
	connCancel context.CancelFunc

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
		reconnectC:  make(chan struct{}, 1),
	}

	stream.OnDepthEvent(func(e *DepthEvent) {
		if debugBinanceDepth {
			log.Infof("received %s depth event updateID %d ~ %d (len %d)", e.Symbol, e.FirstUpdateID, e.FinalUpdateID, e.FinalUpdateID-e.FirstUpdateID)
		}

		f, ok := stream.depthFrames[e.Symbol]
		if !ok {
			f = &DepthFrame{
				client:  client,
				context: context.Background(),
				Symbol:  e.Symbol,
				resetC:  make(chan struct{}, 1),
			}

			stream.depthFrames[e.Symbol] = f

			f.OnReady(func(snapshotDepth DepthEvent, bufEvents []DepthEvent) {
				log.Infof("depth snapshot ready: %s", snapshotDepth.String())

				snapshot, err := snapshotDepth.OrderBook()
				if err != nil {
					log.WithError(err).Error("book snapshot convert error")
					return
				}

				if valid, err := snapshot.IsValid(); !valid {
					log.Errorf("depth snapshot is invalid, event: %+v, error: %v", snapshotDepth, err)
				}

				stream.EmitBookSnapshot(snapshot)

				for _, e := range bufEvents {
					bookUpdate, err := e.OrderBook()
					if err != nil {
						log.WithError(err).Error("book convert error")
						return
					}

					stream.EmitBookUpdate(bookUpdate)
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

			order, err := e.Order()
			if err != nil {
				log.WithError(err).Error("order convert error")
				return
			}

			// Update Order with FILLED event
			if order.Status == types.OrderStatusFilled {
				stream.EmitOrderUpdate(*order)
			}
		}
	})

	stream.OnDisconnect(func() {
		log.Infof("resetting depth snapshots...")
		for _, f := range stream.depthFrames {
			f.emitReset()
		}
	})

	stream.OnConnect(func() {
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

	// use the default ping handler
	conn.SetPingHandler(nil)

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

func (s *Stream) emitReconnect() {
	select {
	case s.reconnectC <- struct{}{}:
	default:
	}
}

func (s *Stream) Connect(ctx context.Context) error {
	err := s.connect(ctx)
	if err != nil {
		return err
	}

	// start one re-connector goroutine with the base context
	go s.reconnector(ctx)

	s.EmitStart()
	return nil
}

func (s *Stream) reconnector(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-s.reconnectC:
			// ensure the previous context is cancelled
			if s.connCancel != nil {
				s.connCancel()
			}

			log.Warnf("received reconnect signal, reconnecting...")
			time.Sleep(3 * time.Second)

			if err := s.connect(ctx); err != nil {
				log.WithError(err).Errorf("connect error, try to reconnect again...")
				s.emitReconnect()
			}
		}
	}
}

func (s *Stream) connect(ctx context.Context) error {
	// should only start one connection one time, so we lock the mutex
	s.connLock.Lock()

	// create a new context
	s.connCtx, s.connCancel = context.WithCancel(ctx)

	if s.publicOnly {
		log.Infof("stream is set to public only mode")
	} else {
		log.Infof("request listen key for creating user data stream...")

		listenKey, err := s.fetchListenKey(ctx)
		if err != nil {
			s.connCancel()
			s.connLock.Unlock()
			return err
		}

		s.ListenKey = listenKey
		log.Infof("user data stream created. listenKey: %s", maskListenKey(s.ListenKey))

		go s.listenKeyKeepAlive(s.connCtx, listenKey)
	}

	// when in public mode, the listen key is an empty string
	conn, err := s.dial(s.ListenKey)
	if err != nil {
		s.connCancel()
		s.connLock.Unlock()
		return err
	}

	log.Infof("websocket connected")

	s.Conn = conn
	s.connLock.Unlock()

	s.EmitConnect()

	go s.read(s.connCtx)
	go s.ping(s.connCtx)
	return nil
}

func (s *Stream) ping(ctx context.Context) {
	pingTicker := time.NewTicker(15 * time.Second)
	defer pingTicker.Stop()

	for {
		select {

		case <-ctx.Done():
			log.Info("ping worker stopped")
			return

		case <-pingTicker.C:
			s.connLock.Lock()
			if err := s.Conn.WriteControl(websocket.PingMessage, []byte("hb"), time.Now().Add(3*time.Second)); err != nil {
				log.WithError(err).Error("ping error", err)
				s.emitReconnect()
			}
			s.connLock.Unlock()
		}
	}
}

func (s *Stream) listenKeyKeepAlive(ctx context.Context, listenKey string) {
	keepAliveTicker := time.NewTicker(5 * time.Minute)
	defer keepAliveTicker.Stop()

	// if we exit, we should invalidate the existing listen key
	defer func() {
		log.Info("keepalive worker stopped")
		if err := s.invalidateListenKey(ctx, listenKey); err != nil {
			log.WithError(err).Error("invalidate listen key error")
		}
	}()

	for {
		select {

		case <-ctx.Done():
			return

		case <-keepAliveTicker.C:
			if err := s.keepaliveListenKey(ctx, listenKey); err != nil {
				log.WithError(err).Errorf("listen key keep-alive error: %v key: %s", err, maskListenKey(listenKey))
				s.emitReconnect()
				return
			}

		}
	}
}

func (s *Stream) read(ctx context.Context) {
	defer func() {
		if s.connCancel != nil {
			s.connCancel()
		}
		s.EmitDisconnect()
	}()

	for {
		select {

		case <-ctx.Done():
			return

		default:
			s.connLock.Lock()
			if err := s.Conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				log.WithError(err).Errorf("set read deadline error: %s", err.Error())
			}

			mt, message, err := s.Conn.ReadMessage()
			s.connLock.Unlock()

			if err != nil {
				// if it's a network timeout error, we should re-connect
				switch err := err.(type) {

				// if it's a websocket related error
				case *websocket.CloseError:
					if err.Code == websocket.CloseNormalClosure {
						return
					}

					// for unexpected close error, we should re-connect
					// emit reconnect to start a new connection
					s.emitReconnect()
					return

				case net.Error:
					log.WithError(err).Error("network error")
					s.emitReconnect()
					return

				default:
					log.WithError(err).Error("unexpected connection error")
					s.emitReconnect()
					return
				}
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

	if s.connCancel != nil {
		s.connCancel()
	}

	s.connLock.Lock()
	err := s.Conn.Close()
	s.connLock.Unlock()
	return err
}

func maskListenKey(listenKey string) string {
	maskKey := listenKey[0:5]
	return maskKey + strings.Repeat("*", len(listenKey)-1-5)
}

//go:generate callbackgen -type DepthFrame

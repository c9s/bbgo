package binance

import (
	"context"
	"crypto/ed25519"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/depth"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/sirupsen/logrus"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"

	api "github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/types"
)

// from Binance document:
// The websocket server will send a ping frame every 3 minutes.
// If the websocket server does not receive a pong frame back from the connection within a 10 minute period, the connection will be disconnected.
// Unsolicited pong frames are allowed.

// WebSocket connections have a limit of 5 incoming messages per second. A message is considered:
// A PING frame
// A PONG frame
// A JSON controlled message (e.g. subscribe, unsubscribe)
const listenKeyKeepAliveInterval = 15 * time.Minute

type WebSocketCommand struct {
	// request ID is required
	ID     int      `json:"id"`
	Method string   `json:"method"`
	Params []string `json:"params"`
}

type LoginParams struct {
	APIKey    string `json:"apiKey"`
	Signature string `json:"signature"`
	Timestamp int64  `json:"timestamp"`
}
type LoginCommand struct {
	WebSocketCommand
	Params LoginParams `json:"params"`
}

//go:generate callbackgen -type Stream -interface
type Stream struct {
	types.MarginSettings
	types.FuturesSettings
	types.StandardStream

	client        *binance.Client
	futuresClient *futures.Client
	privateKey    ed25519.PrivateKey
	ed25519Auth   bool

	// custom callbacks
	depthEventCallbacks       []func(e *DepthEvent)
	kLineEventCallbacks       []func(e *KLineEvent)
	kLineClosedEventCallbacks []func(e *KLineEvent)

	marketTradeEventCallbacks []func(e *MarketTradeEvent)
	aggTradeEventCallbacks    []func(e *AggTradeEvent)
	forceOrderEventCallbacks  []func(e *ForceOrderEvent)

	balanceUpdateEventCallbacks           []func(event *BalanceUpdateEvent)
	outboundAccountInfoEventCallbacks     []func(event *OutboundAccountInfoEvent)
	outboundAccountPositionEventCallbacks []func(event *OutboundAccountPositionEvent)
	executionReportEventCallbacks         []func(event *ExecutionReportEvent)
	bookTickerEventCallbacks              []func(event *BookTickerEvent)

	// futures market data stream
	markPriceUpdateEventCallbacks       []func(e *MarkPriceUpdateEvent)
	continuousKLineEventCallbacks       []func(e *ContinuousKLineEvent)
	continuousKLineClosedEventCallbacks []func(e *ContinuousKLineEvent)

	// futures user data stream event callbacks
	orderTradeUpdateEventCallbacks    []func(e *OrderTradeUpdateEvent)
	accountUpdateEventCallbacks       []func(e *AccountUpdateEvent)
	accountConfigUpdateEventCallbacks []func(e *AccountConfigUpdateEvent)
	marginCallEventCallbacks          []func(e *MarginCallEvent)
	listenKeyExpiredCallbacks         []func(e *ListenKeyExpired)

	// depthBuffers is used for storing the depth info
	depthBuffers map[string]*depth.Buffer
}

func NewStream(ex *Exchange, client *binance.Client, futuresClient *futures.Client) *Stream {
	stream := &Stream{
		StandardStream: types.NewStandardStream(),
		client:         client,
		futuresClient:  futuresClient,
		privateKey:     ex.getEd25519PrivateKey(),
		ed25519Auth:    ex.getEd25519Auth(),
		depthBuffers:   make(map[string]*depth.Buffer),
	}

	stream.SetParser(parseWebSocketEvent)
	stream.SetDispatcher(stream.dispatchEvent)
	stream.SetEndpointCreator(stream.createEndpoint)

	stream.OnDepthEvent(func(e *DepthEvent) {
		f, ok := stream.depthBuffers[e.Symbol]
		if !ok {
			f = depth.NewBuffer(func() (types.SliceOrderBook, int64, error) {
				log.Infof("fetching %s depth...", e.Symbol)
				return ex.QueryDepth(context.Background(), e.Symbol)
			}, 3*time.Second)
			f.SetLogger(logrus.WithFields(logrus.Fields{"exchange": "binance", "symbol": e.Symbol, "component": "depthBuffer"}))
			f.OnReady(func(snapshot types.SliceOrderBook, updates []depth.Update) {
				stream.EmitBookSnapshot(snapshot)
				for _, u := range updates {
					stream.EmitBookUpdate(u.Object)
				}
			})
			f.OnPush(func(update depth.Update) {
				stream.EmitBookUpdate(update.Object)
			})
			stream.depthBuffers[e.Symbol] = f
		}

		if err := f.AddUpdate(types.SliceOrderBook{
			Symbol: e.Symbol,
			Time:   e.EventBase.Time.Time(),
			Bids:   e.Bids,
			Asks:   e.Asks,
		}, e.FirstUpdateID, e.FinalUpdateID); err != nil {
			log.WithError(err).Errorf("found missing %s update event", e.Symbol)
		}
	})

	stream.OnOutboundAccountPositionEvent(stream.handleOutboundAccountPositionEvent)
	stream.OnKLineEvent(stream.handleKLineEvent)
	stream.OnBookTickerEvent(stream.handleBookTickerEvent)
	stream.OnExecutionReportEvent(stream.handleExecutionReportEvent)
	stream.OnContinuousKLineEvent(stream.handleContinuousKLineEvent)
	stream.OnMarketTradeEvent(stream.handleMarketTradeEvent)
	stream.OnAggTradeEvent(stream.handleAggTradeEvent)
	stream.OnForceOrderEvent(stream.handleForceOrderEvent)

	// Futures User Data Stream
	// ===================================
	// Event type ACCOUNT_UPDATE from user data stream updates Balance and FuturesPosition.
	stream.OnOrderTradeUpdateEvent(stream.handleOrderTradeUpdateEvent)
	// ===================================

	stream.OnDisconnect(stream.handleDisconnect)
	stream.OnConnect(stream.handleConnect)
	stream.OnListenKeyExpired(func(e *ListenKeyExpired) {
		stream.Reconnect()
	})
	return stream
}

func (s *Stream) handleDisconnect() {
	log.Debugf("resetting depth snapshots...")
	for _, f := range s.depthBuffers {
		f.Reset()
	}
}

func (s *Stream) handleConnect() {
	var err error
	if !s.PublicOnly {
		if s.ed25519Auth {
			err = s.authConnectEd25519()
		} else {
			log.Warnf("Ed25519 private key is not set, using deprecated HMAC authentication")
			err = s.authConnectHMAC()
		}
	} else {
		err = s.writeSubscriptions()
	}
	if err != nil {
		log.WithError(err).Error("subscribe error")
	}
}

func (s *Stream) authConnectHMAC() error {
	// Emit Auth before establishing the connection to prevent the caller from missing the Update data after
	// creating the order.
	// spawn a goroutine to emit auth event to prevent blocking the main event loop
	go s.EmitAuth()
	return nil
}

func (s *Stream) authConnectEd25519() error {
	timestamp := time.Now().UnixNano() / 1e6
	paramString := strings.Join([]string{
		"apiKey=" + s.client.APIKey,
		"timestamp=" + strconv.FormatInt(timestamp, 10),
	}, "&")
	signature := api.GenerateSignatureEd25519(paramString, s.privateKey)
	err := s.Conn.WriteJSON(LoginCommand{
		WebSocketCommand: WebSocketCommand{
			Method: "session.login",
			ID:     1,
		},
		Params: LoginParams{
			APIKey:    s.client.APIKey,
			Signature: signature,
			Timestamp: timestamp,
		},
	})
	if err == nil {
		// Emit auth to notify that the user data stream connection is established
		// spawn a goroutine to prevent blocking the main event loop
		go s.EmitAuth()
	}
	return err
}

// writeSubscriptions send the subscription command to the websocket
// server in order to establish the connection to market data sources
func (s *Stream) writeSubscriptions() error {
	var params []string
	for _, subscription := range s.Subscriptions {
		params = append(params, convertSubscription(subscription))
	}
	if len(params) == 0 {
		return nil
	}
	log.Infof("subscribing channels: %+v", params)
	err := s.Conn.WriteJSON(WebSocketCommand{
		Method: "SUBSCRIBE",
		Params: params,
		ID:     1,
	})

	return err
}

func (s *Stream) handleContinuousKLineEvent(e *ContinuousKLineEvent) {
	kline := e.KLine.KLine()
	if e.KLine.Closed {
		s.EmitContinuousKLineClosedEvent(e)
		s.EmitKLineClosed(kline)
	} else {
		s.EmitKLine(kline)
	}
}

func (s *Stream) handleExecutionReportEvent(e *ExecutionReportEvent) {
	switch e.CurrentExecutionType {

	case "NEW", "CANCELED", "REJECTED", "EXPIRED", "REPLACED":
		order, err := e.Order()
		if err != nil {
			log.WithError(err).Error("order convert error")
			return
		}

		s.EmitOrderUpdate(*order)

	case "TRADE":
		trade, err := e.Trade()
		if err != nil {
			log.WithError(err).Error("trade convert error")
			return
		}

		s.EmitTradeUpdate(*trade)

		order, err := e.Order()
		if err != nil {
			log.WithError(err).Error("order convert error")
			return
		}

		// Update Order with FILLED event
		if order.Status == types.OrderStatusFilled {
			s.EmitOrderUpdate(*order)
		}
	}
}

func (s *Stream) handleBookTickerEvent(e *BookTickerEvent) {
	s.EmitBookTickerUpdate(e.BookTicker())
}

func (s *Stream) handleMarketTradeEvent(e *MarketTradeEvent) {
	s.EmitMarketTrade(e.Trade())
}

func (s *Stream) handleAggTradeEvent(e *AggTradeEvent) {
	s.EmitAggTrade(e.Trade())
}

func (s *Stream) handleForceOrderEvent(e *ForceOrderEvent) {
	s.EmitForceOrder(e.LiquidationInfo())
}

func (s *Stream) handleKLineEvent(e *KLineEvent) {
	kline := e.KLine.KLine()
	if e.KLine.Closed {
		s.EmitKLineClosedEvent(e)
		s.EmitKLineClosed(kline)
	} else {
		s.EmitKLine(kline)
	}
}

func (s *Stream) handleOutboundAccountPositionEvent(e *OutboundAccountPositionEvent) {
	snapshot := types.BalanceMap{}
	for _, balance := range e.Balances {
		snapshot[balance.Asset] = types.Balance{
			Currency:  balance.Asset,
			Available: balance.Free,
			Locked:    balance.Locked,
		}
	}
	s.EmitBalanceSnapshot(snapshot)
}

func (s *Stream) handleOrderTradeUpdateEvent(e *OrderTradeUpdateEvent) {
	switch e.OrderTrade.CurrentExecutionType {

	case "NEW", "CANCELED", "EXPIRED":
		order, err := e.OrderFutures()
		if err != nil {
			log.WithError(err).Error("futures order convert error")
			return
		}

		s.EmitOrderUpdate(*order)

	case "TRADE":
		trade, err := e.TradeFutures()
		if err != nil {
			log.WithError(err).Error("futures trade convert error")
			return
		}

		s.EmitTradeUpdate(*trade)

		order, err := e.OrderFutures()
		if err != nil {
			log.WithError(err).Error("futures order convert error")
			return
		}

		// Update Order with FILLED event
		if order.Status == types.OrderStatusFilled {
			s.EmitOrderUpdate(*order)
		}

	case "CALCULATED - Liquidation Execution":
		log.Infof("CALCULATED - Liquidation Execution not support yet.")
	}

}

func (s *Stream) getEndpointUrl(listenKey string) string {
	var url string

	if s.IsFutures {
		url = FuturesWebSocketURL + "/ws"
	} else if isBinanceUs() {
		url = BinanceUSWebSocketURL + "/ws"
	} else {
		url = WebSocketURL + "/ws"
	}

	if !s.PublicOnly {
		url += "/" + listenKey
	}

	return url
}

func (s *Stream) createEndpoint(ctx context.Context) (string, error) {
	if s.ed25519Auth {
		return s.createEndpointEd25519()
	}
	return s.createEndpointHMAC(ctx)

}

func (s *Stream) createEndpointHMAC(ctx context.Context) (string, error) {
	var err error
	var listenKey string
	if s.PublicOnly {
		log.Debugf("stream is set to public only mode")
	} else {
		listenKey, err = s.fetchListenKey(ctx)
		if err != nil {
			return "", err
		}

		log.Debugf("listen key is created: %s", util.MaskKey(listenKey))
		go s.listenKeyKeepAlive(ctx, listenKey)
	}

	url := s.getEndpointUrl(listenKey)
	return url, nil
}

func (s *Stream) createEndpointEd25519() (string, error) {
	var url string
	if s.PublicOnly {
		// use websocket stream endpoint
		log.Debugf("(ed25519) stream is set to public only mode")
		if s.IsFutures {
			url = FuturesWebSocketURL + "/ws"
		} else if isBinanceUs() {
			url = BinanceUSWebSocketURL + "/ws"
		} else {
			url = WebSocketURL + "/ws"
		}
	} else {
		// new ws-api for user data stream
		url = wsApiWebsocketUrl
	}

	return url, nil
}

func (s *Stream) dispatchEvent(e interface{}) {
	switch e := e.(type) {

	case *OutboundAccountPositionEvent:
		s.EmitOutboundAccountPositionEvent(e)

	case *OutboundAccountInfoEvent:
		s.EmitOutboundAccountInfoEvent(e)

	case *BalanceUpdateEvent:
		s.EmitBalanceUpdateEvent(e)

	case *MarketTradeEvent:
		s.EmitMarketTradeEvent(e)

	case *AggTradeEvent:
		s.EmitAggTradeEvent(e)

	case *KLineEvent:
		s.EmitKLineEvent(e)

	case *BookTickerEvent:
		s.EmitBookTickerEvent(e)

	case *DepthEvent:
		s.EmitDepthEvent(e)

	case *ExecutionReportEvent:
		s.EmitExecutionReportEvent(e)

	case *MarkPriceUpdateEvent:
		s.EmitMarkPriceUpdateEvent(e)

	case *ContinuousKLineEvent:
		s.EmitContinuousKLineEvent(e)

	case *OrderTradeUpdateEvent:
		s.EmitOrderTradeUpdateEvent(e)

	case *AccountUpdateEvent:
		s.EmitAccountUpdateEvent(e)

	case *AccountConfigUpdateEvent:
		s.EmitAccountConfigUpdateEvent(e)

	case *ListenKeyExpired:
		s.EmitListenKeyExpired(e)

	case *ForceOrderEvent:
		s.EmitForceOrderEvent(e)

	case *MarginCallEvent:

	}
}

func (s *Stream) fetchListenKey(ctx context.Context) (string, error) {
	if s.IsMargin {
		if s.IsIsolatedMargin {
			log.Debugf("isolated margin %s is enabled, requesting margin user stream listen key...", s.IsolatedMarginSymbol)
			req := s.client.NewStartIsolatedMarginUserStreamService()
			req.Symbol(s.IsolatedMarginSymbol)
			return req.Do(ctx)
		}

		log.Debugf("margin mode is enabled, requesting margin user stream listen key...")
		req := s.client.NewStartMarginUserStreamService()
		return req.Do(ctx)
	} else if s.IsFutures {
		log.Debugf("futures mode is enabled, requesting futures user stream listen key...")
		req := s.futuresClient.NewStartUserStreamService()
		return req.Do(ctx)
	}

	log.Debugf("spot mode is enabled, requesting user stream listen key...")
	return s.client.NewStartUserStreamService().Do(ctx)
}

func (s *Stream) keepaliveListenKey(ctx context.Context, listenKey string) error {
	log.Debugf("keepalive listen key: %s", util.MaskKey(listenKey))
	if s.IsMargin {
		if s.IsIsolatedMargin {
			req := s.client.NewKeepaliveIsolatedMarginUserStreamService().ListenKey(listenKey)
			req.Symbol(s.IsolatedMarginSymbol)
			return req.Do(ctx)
		}
		req := s.client.NewKeepaliveMarginUserStreamService().ListenKey(listenKey)
		return req.Do(ctx)
	} else if s.IsFutures {
		req := s.futuresClient.NewKeepaliveUserStreamService().ListenKey(listenKey)
		return req.Do(ctx)
	}

	return s.client.NewKeepaliveUserStreamService().ListenKey(listenKey).Do(ctx)
}

func (s *Stream) closeListenKey(ctx context.Context, listenKey string) (err error) {
	// should use background context to invalidate the user stream
	log.Debugf("closing listen key: %s", util.MaskKey(listenKey))

	if s.IsMargin {
		if s.IsIsolatedMargin {
			req := s.client.NewCloseIsolatedMarginUserStreamService().ListenKey(listenKey)
			req.Symbol(s.IsolatedMarginSymbol)
			err = req.Do(ctx)
		} else {
			req := s.client.NewCloseMarginUserStreamService().ListenKey(listenKey)
			err = req.Do(ctx)
		}

	} else if s.IsFutures {
		req := s.futuresClient.NewCloseUserStreamService().ListenKey(listenKey)
		err = req.Do(ctx)
	} else {
		err = s.client.NewCloseUserStreamService().ListenKey(listenKey).Do(ctx)
	}

	return err
}

func (s *Stream) String() string {
	ss := "binance.Stream"

	if s.PublicOnly {
		ss += " (public only)"
	} else {
		ss += " (user data)"
	}

	if s.MarginSettings.IsMargin {
		ss += " (margin)"
	} else if s.FuturesSettings.IsFutures {
		ss += " (futures)"
	}

	return ss
}

// listenKeyKeepAlive
// From Binance
// Keepalive a user data stream to prevent a time out. User data streams will close after 60 minutes.
// It's recommended to send a ping about every 30 minutes.
func (s *Stream) listenKeyKeepAlive(ctx context.Context, listenKey string) {
	keepAliveTicker := time.NewTicker(listenKeyKeepAliveInterval)
	defer keepAliveTicker.Stop()

	// if we exit, we should invalidate the existing listen key
	defer func() {
		log.Debugf("keepalive worker stopped")
		if err := s.closeListenKey(context.Background(), listenKey); err != nil {
			log.WithError(err).Errorf("close listen key error: %v key: %s", err, util.MaskKey(listenKey))
		}
	}()

	log.Debugf("starting listen key keep alive worker with interval %s, listen key = %s", listenKeyKeepAliveInterval, util.MaskKey(listenKey))

	for {
		select {

		case <-s.CloseC:
			return

		case <-ctx.Done():
			return

		case <-keepAliveTicker.C:
			for i := 0; i < 5; i++ {
				err := s.keepaliveListenKey(ctx, listenKey)
				if err != nil {
					time.Sleep(5 * time.Second)
					switch err.(type) {
					case net.Error:
						log.WithError(err).Errorf("listen key keep-alive network error: %v key: %s", err, util.MaskKey(listenKey))
						continue

					default:
						log.WithError(err).Errorf("listen key keep-alive unexpected error: %v key: %s", err, util.MaskKey(listenKey))
						s.Reconnect()
						return

					}
				} else {
					break
				}
			}

		}
	}
}

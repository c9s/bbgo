package binance

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/depth"
	"github.com/c9s/bbgo/pkg/util"

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
	ID     any    `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

type LogonParams struct {
	APIKey    string `json:"apiKey"`
	Signature string `json:"signature"`
	Timestamp int64  `json:"timestamp"`
}

//go:generate callbackgen -type Stream -interface
type Stream struct {
	types.MarginSettings
	types.FuturesSettings
	types.StandardStream

	client        *binance.Client
	futuresClient *futures.Client

	ed25519authentication

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

	errorCallbacks []func(e *ErrorEvent)

	// depthBuffers is used for storing the depth info
	depthBuffers map[string]*depth.Buffer
}

func NewStream(ex *Exchange, client *binance.Client, futuresClient *futures.Client) *Stream {
	stream := &Stream{
		StandardStream: types.NewStandardStream(),
		client:         client,
		futuresClient:  futuresClient,

		ed25519authentication: ex.ed25519authentication,
		MarginSettings:        ex.MarginSettings,
		FuturesSettings:       ex.FuturesSettings,

		depthBuffers: make(map[string]*depth.Buffer),
	}

	stream.SetParser(parseWebSocketEvent)
	stream.SetDispatcher(stream.dispatchEvent)
	stream.SetEndpointCreator(stream.createEndpoint)

	stream.OnError(func(e *ErrorEvent) {
		log.Errorf("ErrorEvent: %+v", e)
	})

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

	if debugMode {
		stream.OnRawMessage(func(raw []byte) {
			log.Info(string(raw))
		})
	}

	stream.OnDisconnect(stream.handleDisconnect)
	stream.OnConnect(stream.handleConnect)
	stream.OnListenKeyExpired(func(e *ListenKeyExpired) {
		log.Warnf("listen key expired, triggering reconnect: %+v", e)
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
	if s.PublicOnly {
		if err := s.writeSubscriptions(); err != nil {
			log.WithError(err).Error("subscribe error")
		}
	} else if s.canUseWsApiEndpoint() {

		if err := s.sendEd25519LoginCommand(); err != nil {
			log.WithError(err).Error("ed25519 auth error")
		}

		time.Sleep(1 * time.Second)

		// futures does
		if !s.IsFutures {
			if err := s.sendSubscribeUserDataStreamCommand(); err != nil {
				log.WithError(err).Error("subscribe user data stream error")
			}
		}

		// TODO: ensure that we receive an authorized event to trigger this auth event
		go s.EmitAuth()
	} else {
		// Emit Auth before establishing the connection to prevent the caller from missing the Update data after
		// creating the order.
		// spawn a goroutine to emit auth event to prevent blocking the main event loop
		go s.EmitAuth()
	}
}

// sendEd25519LoginCommand
// Sample response:
//
//	  {"id":1,"status":200,"result":{"apiKey":".....",
//		  "authorizedSince":1746456891079,
//		  "connectedSince":1746456891107,
//		  "returnRateLimits":true,
//		  "serverTime":1746456891153,"userDataStream":false},
//		"rateLimits":[{"rateLimitType":"REQUEST_WEIGHT","interval":"MINUTE","intervalNum":1,"limit":6000,"count":14}]}
func (s *Stream) sendEd25519LoginCommand() error {
	timestamp := time.Now().UnixMilli()
	params := url.Values{}
	params.Add("apiKey", s.client.APIKey)
	params.Add("timestamp", strconv.FormatInt(timestamp, 10))
	paramsStr := params.Encode()
	signature := api.GenerateSignatureEd25519(paramsStr, s.ed25519authentication.privateKey)
	return s.Conn.WriteJSON(&WebSocketCommand{
		Method: "session.logon",
		ID:     1,
		Params: &LogonParams{
			APIKey:    s.client.APIKey,
			Signature: signature,
			Timestamp: timestamp,
		},
	})
}

func (s *Stream) sendSubscribeUserDataStreamCommand() error {
	return s.Conn.WriteJSON(&WebSocketCommand{
		ID:     2,
		Method: "userDataStream.subscribe",
	})
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

	// TODO: handle subscribe response
	// sample response: {"result":null,"id":2}
	log.Infof("subscribing channels: %+v", params)
	return s.Conn.WriteJSON(&WebSocketCommand{
		Method: "SUBSCRIBE",
		Params: params,
		ID:     2,
	})
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

func (s *Stream) getPublicEndpointUrl() string {
	if s.IsFutures {
		if testNet {
			return TestNetFuturesBaseURL + "/ws"
		}

		return FuturesWebSocketURL + "/ws"
	} else if isBinanceUs() {
		if testNet {
			return BinanceTestBaseURL + "/ws"
		}
		return BinanceUSWebSocketURL + "/ws"
	} else {
		if testNet {
			return TestNetWebSocketURL + "/ws"
		}

		return WebSocketURL + "/ws"
	}
}

func (s *Stream) getUserDataStreamEndpointUrl(listenKey string) string {
	u := s.getPublicEndpointUrl()
	return u + "/" + listenKey
}

// canUseWsApiEndpoint returns true if the stream should use ed25519 authentication with the new ws api endpoint
// only the new spot trading websocket endpoint supports ed25519 authentication
func (s *Stream) canUseWsApiEndpoint() bool {
	// margin and futures don't support ed25519 authentication
	if s.MarginSettings.IsMargin || s.FuturesSettings.IsFutures {
		return false
	}

	return s.ed25519authentication.usingEd25519
}

func (s *Stream) createEndpoint(ctx context.Context) (string, error) {
	if s.PublicOnly {
		return s.getPublicEndpointUrl(), nil
	}

	if s.canUseWsApiEndpoint() {
		// The new websocket api is only for spot trading
		if testNet {
			return WsTestNetWebSocketURL, nil
		}

		return WsSpotWebSocketURL, nil
	}

	return s.createUserDataStreamEndpoint(ctx)
}

func (s *Stream) createUserDataStreamEndpoint(ctx context.Context) (string, error) {
	listenKey, err := s.fetchListenKey(ctx)
	if err != nil {
		return "", err
	}

	debug("listen key is created, starting listen key keep alive worker: %s", util.MaskKey(listenKey))
	go s.listenKeyKeepAlive(ctx, listenKey)

	return s.getUserDataStreamEndpointUrl(listenKey), nil
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
			debug("isolated margin %s is enabled, requesting margin user stream listen key...", s.IsolatedMarginSymbol)
			req := s.client.NewStartIsolatedMarginUserStreamService()
			req.Symbol(s.IsolatedMarginSymbol)
			return req.Do(ctx)
		}

		debug("margin mode is enabled, requesting margin user stream listen key...")
		req := s.client.NewStartMarginUserStreamService()
		return req.Do(ctx)
	} else if s.IsFutures {
		debug("futures mode is enabled, requesting futures user stream listen key...")
		req := s.futuresClient.NewStartUserStreamService()
		return req.Do(ctx)
	}

	debug("spot mode is enabled, requesting user stream listen key...")
	return s.client.NewStartUserStreamService().Do(ctx)
}

func (s *Stream) keepaliveListenKey(ctx context.Context, listenKey string) error {
	debug("keepalive listen key: %s", util.MaskKey(listenKey))
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
	debug("closing listen key: %s", util.MaskKey(listenKey))

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

func toJson(a any) string {
	o, _ := json.MarshalIndent(a, "", " ")
	return string(o)
}

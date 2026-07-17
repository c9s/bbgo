package binance

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/depth"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/util"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"

	"github.com/c9s/bbgo/pkg/types"
)

// IMPORTANT DEPRECATION NOTICE (2025-10-06):
// Receiving user data stream on wss://stream.binance.com:9443 using a listenKey is now deprecated.
// This feature will be removed from Binance's system at a later date.
// Users are recommended to move to the new listen token subscription method:
// 1. POST /sapi/v1/userListenToken: Create a new user data stream and return a listenToken
// 2. WebSocket API method userDataStream.subscribe.listenToken: Subscribe to the user data stream using listenToken
//
// To enable the new method, call exchange.EnableListenToken() before creating the stream.

// from Binance document:
// The websocket server will send a ping frame every 3 minutes.
// If the websocket server does not receive a pong frame back from the connection within a 10 minute period, the connection will be disconnected.
// Unsolicited pong frames are allowed.

// WebSocket connections have a limit of 5 incoming messages per second. A message is considered:
// A PING frame
// A PONG frame
// A JSON controlled message (e.g. subscribe, unsubscribe)

// DEPRECATED (2025-10-06): listenKey keep alive is deprecated, use new listenToken method
const listenKeyKeepAliveInterval = 15 * time.Minute
const listenTokenKeepAliveInterval = 15 * time.Minute

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

type ListenTokenSubscribeParams struct {
	ListenToken string `json:"listenToken"`
}

type RiskBalance struct {
	Currency string
	Borrowed fixedpoint.Value
	Interest fixedpoint.Value
}

//go:generate callbackgen -type Stream -interface
type Stream struct {
	exchange *Exchange

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
	indexPriceKLineEventCallbacks       []func(e *IndexPriceKLineEvent)
	indexPriceKLineCallbacks            []func(kline types.KLine)
	indexPriceKLineClosedCallbacks      []func(kline types.KLine)

	// futures user data stream event callbacks
	orderTradeUpdateEventCallbacks    []func(e *OrderTradeUpdateEvent)
	accountUpdateEventCallbacks       []func(e *AccountUpdateEvent)
	accountConfigUpdateEventCallbacks []func(e *AccountConfigUpdateEvent)
	marginCallEventCallbacks          []func(e *MarginCallEvent)
	listenKeyExpiredCallbacks         []func(e *ListenKeyExpired)
	algoOrderUpdateEventCallbacks     []func(e *AlgoOrderUpdateEvent)
	serverShutdownEventCallbacks      []func(e *ServerShutdownEvent)

	errorCallbacks []func(e *ErrorEvent)

	// depthBuffers is used for storing the depth info
	depthBuffers map[string]*depth.Buffer

	riskBalances          map[string]*RiskBalance
	riskBalancesUpdatedAt time.Time
	mu                    sync.RWMutex

	// New listenToken support (recommended as of 2025-10-06)
	listenToken           string
	listenTokenExpiration time.Time
	once                  util.Reonce

	// futuresAuxStream handles /public subscriptions (depth, bookTicker) when the main
	// stream connects to /market. Only created for futures PublicOnly streams with mixed
	// subscription types. nil when not needed.
	futuresAuxStream  *types.StandardStream
	futuresPublicSubs []types.Subscription // cached /public subscriptions
	futuresMarketSubs []types.Subscription // cached /market subscriptions
}

func NewStream(ex *Exchange, client *binance.Client, futuresClient *futures.Client) *Stream {
	stream := &Stream{
		StandardStream:        types.NewStandardStream(),
		client:                client,
		futuresClient:         futuresClient,
		ed25519authentication: ex.ed25519authentication,

		exchange:     ex,
		depthBuffers: make(map[string]*depth.Buffer),

		riskBalances: make(map[string]*RiskBalance),
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
				log.Debugf("fetching %s depth...", e.Symbol)
				return ex.QueryDepth(context.Background(), e.Symbol)
			}, 3*time.Second)

			if ex.IsFutures {
				f.CheckPreviousID()
			}

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
		}, e.FirstUpdateID, e.FinalUpdateID, e.PreviousUpdateID); err != nil {
			log.WithError(err).Errorf("found missing %s update event", e.Symbol)
		}
	})

	stream.OnOutboundAccountPositionEvent(stream.handleOutboundAccountPositionEvent)
	stream.OnKLineEvent(stream.handleKLineEvent)
	stream.OnBookTickerEvent(stream.handleBookTickerEvent)
	stream.OnExecutionReportEvent(stream.handleExecutionReportEvent)
	stream.OnContinuousKLineEvent(stream.handleContinuousKLineEvent)
	stream.OnIndexPriceKLineEvent(stream.handleIndexPriceKLineEvent)
	stream.OnMarketTradeEvent(stream.handleMarketTradeEvent)
	stream.OnAggTradeEvent(stream.handleAggTradeEvent)
	stream.OnForceOrderEvent(stream.handleForceOrderEvent)

	// Futures User Data Stream
	// ===================================
	// Event type ACCOUNT_UPDATE from user data stream updates Balance and FuturesPosition.
	stream.OnOrderTradeUpdateEvent(stream.handleOrderTradeUpdateEvent)
	// Event type ALGO_UPDATE from user data stream updates AlgoOrder.
	stream.OnAlgoOrderUpdateEvent(stream.handleAlgoOrderUpdateEvent)
	// ===================================

	if debugMode {
		stream.OnRawMessage(func(raw []byte) {
			log.Info(string(raw))
		})
	}

	stream.OnDisconnect(stream.handleDisconnect)
	stream.OnConnect(stream.handleConnect)
	stream.SetBeforeConnect(stream.handleBeforeConnect)
	stream.OnListenKeyExpired(func(e *ListenKeyExpired) {
		log.Warnf("listen key expired, triggering reconnect: %+v", e)
		stream.Reconnect()
	})
	stream.OnServerShutdownEvent(func(e *ServerShutdownEvent) {
		log.Warnf("the server is about to shutdown soon, triggering reconnect: %+v", e)
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

func (s *Stream) updateRiskBalance(ctx context.Context) error {
	if !s.exchange.IsMargin {
		return nil
	}

	account, err := s.exchange.QueryCrossMarginAccount(ctx)
	if err != nil {
		return err
	}

	marginBalances := account.Balances()

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range marginBalances {
		s.riskBalances[b.Currency] = &RiskBalance{
			Currency: b.Currency,
			Borrowed: b.Borrowed,
			Interest: b.Interest,
		}
	}
	s.riskBalancesUpdatedAt = time.Now()

	return nil
}

func (s *Stream) handleBeforeConnect(ctx context.Context) error {
	if s.exchange.IsMargin {
		if err := s.updateRiskBalance(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Close closes the stream and, if present, the auxiliary futures /public stream.
func (s *Stream) Close() error {
	s.ConnLock.Lock()
	auxStream := s.futuresAuxStream
	s.ConnLock.Unlock()

	if auxStream != nil {
		if err := auxStream.Close(); err != nil {
			log.WithError(err).Warn("futures aux stream close error")
		}
	}
	return s.StandardStream.Close()
}

// writeSubscriptions sends a SUBSCRIBE command for all of s.Subscriptions.
func (s *Stream) writeSubscriptions() error {
	return s.writeSpecificSubscriptions(s.Conn, s.Subscriptions)
}

// initFuturesAuxStream creates the auxiliary StandardStream that connects to the futures
// /public endpoint. It shares the same parser and dispatcher as the main stream so that
// events are delivered through the same callback system.
// Only called once; handleConnect guards with futuresAuxStream != nil check.
func (s *Stream) initFuturesAuxStream() {
	aux := types.NewStandardStream()
	aux.SetParser(parseWebSocketEvent)
	aux.SetDispatcher(s.dispatchEvent)
	aux.SetEndpointCreator(func(ctx context.Context) (string, error) {
		return s.getFuturesPublicEndpointUrl(), nil
	})
	aux.OnConnect(func() {
		if err := s.writeSpecificSubscriptions(aux.Conn, s.futuresPublicSubs); err != nil {
			log.WithError(err).Error("futures aux stream /public subscribe error")
		}
	})
	s.futuresAuxStream = &aux
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

func (s *Stream) handleIndexPriceKLineEvent(e *IndexPriceKLineEvent) {
	kline := e.KLine.KLine()
	kline.Symbol = e.Symbol // "k.s" is a Binance placeholder, not the real pair
	if e.KLine.Closed {
		s.EmitIndexPriceKLineClosed(kline)
	} else {
		s.EmitIndexPriceKLine(kline)
	}
}

func (s *Stream) handleExecutionReportEvent(e *ExecutionReportEvent) {
	switch e.CurrentExecutionType {

	case "NEW", "CANCELED", "REJECTED", "EXPIRED", "REPLACED":
		order, err := e.Order(s.exchange.IsMargin, s.exchange.IsIsolatedMargin)
		if err != nil {
			log.WithError(err).Errorf("order convert error: %+v", e)
			return
		}

		s.EmitOrderUpdate(*order)

	case "TRADE":
		trade, err := e.Trade(
			s.exchange.IsMargin,
			s.exchange.IsIsolatedMargin,
		)
		if err != nil {
			log.WithError(err).Errorf("trade convert error: %+v", e)
			return
		}

		s.EmitTradeUpdate(*trade)

		order, err := e.Order(
			s.exchange.IsMargin,
			s.exchange.IsIsolatedMargin,
		)
		if err != nil {
			log.WithError(err).Errorf("order convert error: %+v", e)
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

func (s *Stream) addRiskBalance(balance types.Balance) types.Balance {
	s.mu.RLock()
	rb, hasRb := s.riskBalances[balance.Currency]
	s.mu.RUnlock()

	if hasRb {
		balance.Borrowed = rb.Borrowed
		balance.Interest = rb.Interest
		return balance
	}

	return balance
}

func (s *Stream) handleOutboundAccountPositionEvent(e *OutboundAccountPositionEvent) {
	if time.Since(s.riskBalancesUpdatedAt) > 1*time.Minute {
		if err := s.updateRiskBalance(context.Background()); err != nil {
			log.WithError(err).Error("update risk balance error")
		}
	}

	snapshot := make(types.BalanceMap, len(e.Balances))
	for _, balance := range e.Balances {
		snapshot[balance.Asset] = s.addRiskBalance(types.Balance{
			Currency:  balance.Asset,
			Available: balance.Free,
			Locked:    balance.Locked,
			Borrowed:  fixedpoint.Zero,
			Interest:  fixedpoint.Zero,
		})
	}
	s.EmitBalanceSnapshot(snapshot)
}

func (s *Stream) handleOrderTradeUpdateEvent(e *OrderTradeUpdateEvent) {
	switch e.OrderTrade.CurrentExecutionType {

	case "TRADE_PREVENTION", "REJECTED":
		log.Warnf("ExecutionReport %s: %+v", e.OrderTrade.CurrentExecutionType, e)

	case "NEW", "CANCELED", "EXPIRED":
		order, err := e.OrderFutures(s.exchange.IsIsolatedFutures)
		if err != nil {
			log.WithError(err).Error("futures order convert error")
			return
		}

		s.EmitOrderUpdate(*order)

	case "TRADE":
		trade, err := e.TradeFutures(s.exchange.IsIsolatedFutures)
		if err != nil {
			log.WithError(err).Error("futures trade convert error")
			return
		}

		s.EmitTradeUpdate(*trade)

		order, err := e.OrderFutures(s.exchange.IsIsolatedFutures)
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
	if s.exchange.IsFutures {
		s.futuresPublicSubs, s.futuresMarketSubs = classifyFuturesSubscriptions(s.Subscriptions)
		if len(s.futuresPublicSubs) > 0 && len(s.futuresMarketSubs) > 0 {
			// Mixed: main stream → /market, aux stream → /public (init once)
			if s.futuresAuxStream == nil {
				s.initFuturesAuxStream()
			}

			return s.getFuturesMarketEndpointUrl()
		} else if len(s.futuresPublicSubs) > 0 {

			// Only depth/bookTicker subscriptions
			return s.getFuturesPublicEndpointUrl()
		}

		// Only market subscriptions (or no subscriptions yet)
		return s.getFuturesMarketEndpointUrl()
	} else if isBinanceUs() {
		if testNet {
			return BinanceTestBaseURL + "/ws"
		}
		return BinanceUSWebSocketURL + "/ws"
	} else {
		if testNet {
			return TestNetWebSocketURL + "/ws"
		}

		// Use deprecated WebSocket URL for backward compatibility
		// Note: wss://stream.binance.com:9443 with listenKey is deprecated as of 2025-10-06
		return WebSocketURL + "/ws"
	}
}

func (s *Stream) getUserDataStreamEndpointUrl(listenKey string) string {
	if s.exchange.IsFutures {
		return s.getFuturesPrivateEndpointUrl(listenKey)
	}

	u := s.getPublicEndpointUrl()
	return u + "/" + listenKey
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
	// Use new listenToken method (recommended as of 2025-10-06)
	if s.exchange.IsMargin && !s.exchange.useListenKey {
		return s.createUserDataStreamEndpointWithListenToken(ctx)
	}

	return s.createUserDataStreamEndpointWithListenKey(ctx)
}

func (s *Stream) createUserDataStreamEndpointWithListenKey(ctx context.Context) (string, error) {
	// Use deprecated listenKey method
	listenKey, err := s.fetchListenKey(ctx)
	if err != nil {
		return "", err
	}
	// listenKey is empty -> spot trading
	if listenKey == "" {
		return WsSpotWebSocketURL, nil
	}

	debug("listen key is created, starting listen key keep alive worker: %s", util.MaskKey(listenKey))
	go s.listenKeyKeepAlive(ctx, listenKey)

	return s.getUserDataStreamEndpointUrl(listenKey), nil
}

func (s *Stream) createUserDataStreamEndpointWithListenToken(ctx context.Context) (string, error) {
	listenToken, expiration, err := s.fetchListenToken(ctx)
	if err != nil {
		return "", err
	}

	s.listenToken = listenToken
	s.listenTokenExpiration = expiration

	debug("listenToken created, expires at: %s, starting token refresh worker", expiration)
	s.once.Do(func() {
		go s.listenTokenRefreshWorker(ctx)
	})

	// Use the new WebSocket API endpoint with listenToken subscription
	if testNet {
		return WsTestNetWebSocketURL, nil
	}
	return WsSpotWebSocketURL, nil
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

	case *IndexPriceKLineEvent:
		s.EmitIndexPriceKLineEvent(e)

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
	case *AlgoOrderUpdateEvent:
		s.EmitAlgoOrderUpdateEvent(e)
	case *ServerShutdownEvent:
		s.EmitServerShutdownEvent(e)

	case *MarginCallEvent:
	}
}

// fetchListenToken creates a new user data stream using the new listenToken method (2025-10-06 recommended)
func (s *Stream) fetchListenToken(ctx context.Context) (string, time.Time, error) {
	if s.exchange.useListenKey {
		return "", time.Time{}, fmt.Errorf("listenKey method is enabled, cannot use listenToken")
	}

	// Use the binanceapi client to create listenToken
	req := s.exchange.client2.NewCreateMarginAccountListenTokenRequest()

	if s.exchange.IsMargin && s.exchange.IsIsolatedMargin {
		req.Symbol(s.exchange.IsolatedMarginSymbol)
		req.IsIsolated(true)
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to fetch listenToken: %w", err)
	}

	expiration := resp.ExpirationTime.Time()
	debug("listenToken created, expires at: %s, token: %s", expiration, util.MaskKey(resp.Token))

	return resp.Token, expiration, nil
}

func (s *Stream) fetchListenKey(ctx context.Context) (string, error) {
	if s.exchange.IsMargin {
		if s.exchange.IsIsolatedMargin {
			debug("isolated margin %s is enabled, requesting margin user stream listen key...", s.exchange.IsolatedMarginSymbol)
			req := s.client.NewStartIsolatedMarginUserStreamService()
			req.Symbol(s.exchange.IsolatedMarginSymbol)
			return req.Do(ctx)
		}

		debug("margin mode is enabled, requesting margin user stream listen key...")
		req := s.client.NewStartMarginUserStreamService()
		return req.Do(ctx)
	} else if s.exchange.IsFutures {
		debug("futures mode is enabled, requesting futures user stream listen key...")
		req := s.futuresClient.NewStartUserStreamService()
		return req.Do(ctx)
	}

	// listenKey is removed for spot trading (POST /api/v3/userDataStream)
	// See: https://www.binance.com/en/support/announcement/detail/80b38c8954bf4965b21eeb3c5fc6edfa
	debug("spot mode is enabled, requesting user stream listen key...")
	// return s.client.NewStartUserStreamService().Do(ctx)
	return "", nil
}

func (s *Stream) keepaliveListenKey(ctx context.Context, listenKey string) error {
	debug("keepalive listen key: %s", util.MaskKey(listenKey))
	if s.exchange.IsMargin {
		if s.exchange.IsIsolatedMargin {
			req := s.client.NewKeepaliveIsolatedMarginUserStreamService().ListenKey(listenKey)
			req.Symbol(s.exchange.IsolatedMarginSymbol)
			return req.Do(ctx)
		}
		req := s.client.NewKeepaliveMarginUserStreamService().ListenKey(listenKey)
		return req.Do(ctx)
	} else if s.exchange.IsFutures {
		req := s.futuresClient.NewKeepaliveUserStreamService().ListenKey(listenKey)
		return req.Do(ctx)
	}

	return s.client.NewKeepaliveUserStreamService().ListenKey(listenKey).Do(ctx)
}

func (s *Stream) closeListenKey(ctx context.Context, listenKey string) (err error) {
	// should use background context to invalidate the user stream
	debug("closing listen key: %s", util.MaskKey(listenKey))

	if s.exchange.IsMargin {
		if s.exchange.IsIsolatedMargin {
			req := s.client.NewCloseIsolatedMarginUserStreamService().ListenKey(listenKey)
			req.Symbol(s.exchange.IsolatedMarginSymbol)
			err = req.Do(ctx)
		} else {
			req := s.client.NewCloseMarginUserStreamService().ListenKey(listenKey)
			err = req.Do(ctx)
		}

	} else if s.exchange.IsFutures {
		req := s.futuresClient.NewCloseUserStreamService().ListenKey(listenKey)
		err = req.Do(ctx)
	} else {
		err = s.client.NewCloseUserStreamService().ListenKey(listenKey).Do(ctx)
	}

	return err
}

// listenTokenRefreshWorker manages the lifecycle of listenToken for the new WebSocket API method
func (s *Stream) listenTokenRefreshWorker(ctx context.Context) {
	log.Info("listenToken refresh worker started")

	defer func() {
		log.Info("listenToken refresh worker exiting, resetting once...")
		s.once.Reset()
	}()

	// Set timeout to 3 times the keep-alive interval to ensure we refresh the token before it expires
	timeout := listenTokenKeepAliveInterval * 3
	ticker := time.NewTicker(listenTokenKeepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.CloseC:
			log.Info("listenToken refresh worker exiting due to stream close")
			return
		case <-ctx.Done():
			log.Info("listenToken refresh worker exiting due to context done")
			return
		case <-ticker.C:
			log.Infof("checking listenToken expiration (%s)...", s.listenTokenExpiration)
			if time.Until(s.listenTokenExpiration) > timeout {
				continue
			}

			log.Info("listenToken is about to expire, refreshing...")

			// Refresh the listenToken
			newToken, newExpiration, err := s.fetchListenToken(ctx)
			if err != nil {
				log.WithError(err).Error("failed to refresh listenToken, will retry in 1 minute")
				ticker.Reset(time.Minute)
				continue
			}

			s.listenToken = newToken
			s.listenTokenExpiration = newExpiration

			if err := s.sendListenTokenSubscribeCommand(); err != nil {
				log.WithError(err).Error("listenToken subscribe error")
				continue
			}
		}
	}
}

func (s *Stream) String() string {
	ss := "binance.Stream"

	if s.PublicOnly {
		ss += " (public only)"
	} else {
		ss += " (user data)"
	}

	if s.exchange.MarginSettings.IsMargin {
		ss += " (margin)"
	} else if s.exchange.FuturesSettings.IsFutures {
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
		if err := s.closeListenKey(context.Background(), listenKey); err != nil {
			log.WithError(err).Warnf("close listen key error: %v key: %s", err, util.MaskKey(listenKey))
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

func (s *Stream) handleAlgoOrderUpdateEvent(e *AlgoOrderUpdateEvent) {
	order, err := e.OrderFutures(
		s.exchange.IsFutures,
		s.exchange.IsMargin,
		s.exchange.IsIsolatedFutures || s.exchange.IsIsolatedMargin,
	)
	if err != nil {
		log.WithError(err).Error("algo order convert error")
		return
	}

	s.EmitOrderUpdate(*order)
}

// isFuturesPublicChannel reports whether a channel belongs to the /public endpoint.
// The /public endpoint handles high-frequency data: depth and bookTicker.
// All other channels belong to the /market endpoint.
func isFuturesPublicChannel(ch types.Channel) bool {
	switch ch {
	case types.BookChannel, types.BookTickerChannel:
		return true
	}
	return false
}

// classifyFuturesSubscriptions splits subscriptions into /public and /market groups.
func classifyFuturesSubscriptions(subs []types.Subscription) (public, market []types.Subscription) {
	for _, s := range subs {
		if isFuturesPublicChannel(s.Channel) {
			public = append(public, s)
		} else {
			market = append(market, s)
		}
	}
	return
}

func (s *Stream) getFuturesPublicEndpointUrl() string {
	if testNet {
		return TestNetFuturesPublicWebSocketURL + "/ws"
	}
	return FuturesPublicWebSocketURL + "/ws"
}

func (s *Stream) getFuturesMarketEndpointUrl() string {
	if testNet {
		return TestNetFuturesMarketWebSocketURL + "/ws"
	}
	return FuturesMarketWebSocketURL + "/ws"
}

// futuresPrivateStreamEvents lists the user data stream events to subscribe on the new
// /private endpoint. The &events= parameter is required — omitting it causes Binance to
// push listenKeyExpired immediately after connecting.
// Ref: https://developers.binance.com/docs/derivatives/usds-margined-futures/websocket-market-streams/Important-WebSocket-Change-Notice
const futuresPrivateStreamEvents = "ORDER_TRADE_UPDATE/TRADE_LITE/ACCOUNT_UPDATE/ACCOUNT_CONFIG_UPDATE/MARGIN_CALL/ALGO_UPDATE"

func (s *Stream) getFuturesPrivateEndpointUrl(listenKey string) string {
	base := FuturesPrivateWebSocketURL
	if testNet {
		base = TestNetFuturesPrivateWebSocketURL
	}
	return base + "/ws?listenKey=" + listenKey + "&events=" + futuresPrivateStreamEvents
}

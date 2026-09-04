package backpack

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/depth"
	"github.com/c9s/bbgo/pkg/exchange/backpack/backpackapi"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/types"
)

// depthSnapshotBufferingPeriod is how long depth updates are buffered while the REST snapshot
// is fetched.
const depthSnapshotBufferingPeriod = 3 * time.Second

// orderLogLimiter keeps a malformed feed from flooding the log.
var orderLogLimiter = rate.NewLimiter(rate.Every(time.Minute), 1)

// interface implementations, checked at compile time
var (
	_ types.Stream       = (*Stream)(nil)
	_ types.Unsubscriber = (*Stream)(nil)
)

//go:generate callbackgen -type Stream -interface
type Stream struct {
	types.StandardStream

	exchange *Exchange
	client   *backpackapi.RestClient

	logger logrus.FieldLogger

	// depthBuffers holds one order book buffer per symbol, keyed by the global symbol.
	depthBuffers map[string]*depth.Buffer

	// native event callbacks
	bookTickerEventCallbacks    []func(e *backpackapi.BookTickerEvent)
	depthEventCallbacks         []func(e *backpackapi.DepthEvent)
	tradeEventCallbacks         []func(e *backpackapi.TradeEvent)
	tickerEventCallbacks        []func(e *backpackapi.TickerEvent)
	kLineEventCallbacks         []func(e *backpackapi.KLineEvent)
	markPriceEventCallbacks     []func(e *backpackapi.MarkPriceEvent)
	balanceUpdateEventCallbacks []func(e *backpackapi.BalanceUpdateEvent)
	orderUpdateEventCallbacks   []func(e *backpackapi.OrderUpdateEvent)
	errorEventCallbacks         []func(e *backpackapi.WebsocketError)
}

func NewStream(ex *Exchange, client *backpackapi.RestClient) *Stream {
	s := &Stream{
		StandardStream: types.NewStandardStream(),
		exchange:       ex,
		client:         client,
		depthBuffers:   make(map[string]*depth.Buffer),
		logger:         log.WithField("module", "stream"),
	}

	s.SetParser(backpackapi.ParseWebsocketMessage)
	s.SetDispatcher(s.dispatchEvent)
	s.SetEndpointCreator(s.createEndpoint)

	s.OnBookTickerEvent(s.handleBookTickerEvent)
	s.OnDepthEvent(s.handleDepthEvent)
	s.OnTradeEvent(s.handleTradeEvent)
	s.OnKLineEvent(s.handleKLineEvent)
	s.OnBalanceUpdateEvent(s.handleBalanceUpdateEvent)
	s.OnOrderUpdateEvent(s.handleOrderUpdateEvent)
	s.OnErrorEvent(s.handleErrorEvent)

	s.OnConnect(s.handleConnect)
	s.OnDisconnect(s.handleDisconnect)

	return s
}

// createEndpoint returns the websocket endpoint. Backpack serves the public and the private
// streams from the same URL; the private ones are told apart by the signed subscribe request.
func (s *Stream) createEndpoint(ctx context.Context) (string, error) {
	return backpackapi.WebSocketURL, nil
}

func (s *Stream) handleConnect() {
	if s.PublicOnly {
		s.subscribePublicStreams()
		return
	}

	s.subscribePrivateStreams()
}

func (s *Stream) subscribePublicStreams() {
	var streams []string
	for _, subscription := range s.Subscriptions {
		stream, err := s.convertSubscription(subscription)
		if err != nil {
			s.logger.WithError(err).Errorf("subscription convert error, subscription: %+v", subscription)
			continue
		}

		streams = append(streams, stream)
	}

	if len(streams) == 0 {
		return
	}

	s.logger.Infof("subscribing to the public streams: %v", streams)

	if err := s.Conn.WriteJSON(backpackapi.WebsocketRequest{
		Method: backpackapi.WebsocketMethodSubscribe,
		Params: streams,
	}); err != nil {
		s.logger.WithError(err).Error("public stream subscribe error")
	}
}

func (s *Stream) subscribePrivateStreams() {
	streams := []string{
		backpackapi.StreamOrderUpdate,
		backpackapi.StreamBalanceUpdate,
	}

	req, err := s.client.NewWebsocketSubscribeRequest(streams)
	if err != nil {
		s.logger.WithError(err).Error("unable to build the private subscribe request")
		return
	}

	s.logger.Infof("subscribing to the private streams: %v", streams)

	if err := s.Conn.WriteJSON(req); err != nil {
		s.logger.WithError(err).Error("private stream subscribe error")
		return
	}

	// the server acknowledges a successful subscribe with silence, and only sends a frame when
	// it rejects the request, so the auth event is emitted optimistically here
	go func() {
		s.EmitAuth()
		s.emitBalanceSnapshot()
	}()
}

// emitBalanceSnapshot pulls the balances over REST, since the balance stream only reports
// changes and never the current state.
func (s *Stream) emitBalanceSnapshot() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var balances types.BalanceMap
	err := retry.GeneralBackoff(ctx, func() (err error) {
		balances, err = s.exchange.QueryAccountBalances(ctx)
		return err
	})

	if err != nil {
		s.logger.WithError(err).Error("no more attempts to retrieve the balances")
		return
	}

	s.EmitBalanceSnapshot(balances)
}

func (s *Stream) handleDisconnect() {
	// the depth buffers hold a snapshot that the next connection's update ids will not line up
	// with, so they have to start over
	for _, buffer := range s.depthBuffers {
		buffer.Reset()
	}
}

// convertSubscription maps a bbgo subscription onto a Backpack stream name.
func (s *Stream) convertSubscription(subscription types.Subscription) (string, error) {
	localSymbol := s.exchange.getLocalSymbol(subscription.Symbol)

	switch subscription.Channel {
	case types.BookChannel:
		return backpackapi.StreamDepth + "." + localSymbol, nil

	case types.BookTickerChannel:
		return backpackapi.StreamBookTicker + "." + localSymbol, nil

	case types.MarketTradeChannel:
		return backpackapi.StreamTrade + "." + localSymbol, nil

	case types.TickerChannel:
		return backpackapi.StreamTicker + "." + localSymbol, nil

	case types.MarkPriceChannel:
		return backpackapi.StreamMarkPrice + "." + localSymbol, nil

	case types.KLineChannel:
		interval, err := toLocalInterval(subscription.Options.Interval)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("%s.%s.%s", backpackapi.StreamKLine, interval, localSymbol), nil
	}

	return "", fmt.Errorf("unsupported stream channel: %s", subscription.Channel)
}

func (s *Stream) dispatchEvent(e interface{}) {
	switch event := e.(type) {
	case *backpackapi.WebsocketError:
		s.EmitErrorEvent(event)

	case *backpackapi.BookTickerEvent:
		s.EmitBookTickerEvent(event)

	case *backpackapi.DepthEvent:
		s.EmitDepthEvent(event)

	case *backpackapi.TradeEvent:
		s.EmitTradeEvent(event)

	case *backpackapi.TickerEvent:
		s.EmitTickerEvent(event)

	case *backpackapi.KLineEvent:
		s.EmitKLineEvent(event)

	case *backpackapi.MarkPriceEvent:
		s.EmitMarkPriceEvent(event)

	case *backpackapi.BalanceUpdateEvent:
		s.EmitBalanceUpdateEvent(event)

	case *backpackapi.OrderUpdateEvent:
		s.EmitOrderUpdateEvent(event)
	}
}

func (s *Stream) handleErrorEvent(e *backpackapi.WebsocketError) {
	s.logger.Errorf("websocket error frame: %v", e)
}

func (s *Stream) handleBookTickerEvent(e *backpackapi.BookTickerEvent) {
	s.EmitBookTickerUpdate(bookTickerEventToGlobalBookTicker(e))
}

// handleDepthEvent feeds the incremental update into the order book buffer of the symbol.
//
// Backpack sends Binance style updates carrying a first/last update id range, so the buffer
// fetches a REST snapshot and replays the buffered updates on top of it.
func (s *Stream) handleDepthEvent(e *backpackapi.DepthEvent) {
	symbol := toGlobalSymbol(e.Symbol)

	buffer, ok := s.depthBuffers[symbol]
	if !ok {
		buffer = depth.NewBuffer(func() (types.SliceOrderBook, int64, error) {
			return s.exchange.QueryDepth(context.Background(), symbol)
		}, depthSnapshotBufferingPeriod)

		buffer.SetLogger(s.logger.WithField("component", "depthBuffer").WithField("symbol", symbol))

		buffer.OnReady(func(snapshot types.SliceOrderBook, updates []depth.Update) {
			s.EmitBookSnapshot(snapshot)
			for _, u := range updates {
				s.EmitBookUpdate(u.Object)
			}
		})

		buffer.OnPush(func(update depth.Update) {
			s.EmitBookUpdate(update.Object)
		})

		s.depthBuffers[symbol] = buffer
	}

	if err := buffer.AddUpdate(depthEventToGlobalOrderBook(e), e.FirstUpdateId, e.LastUpdateId); err != nil {
		s.logger.WithError(err).Warnf("found a missing %s depth update", symbol)
	}
}

func (s *Stream) handleTradeEvent(e *backpackapi.TradeEvent) {
	s.EmitMarketTrade(tradeEventToGlobalTrade(e))
}

func (s *Stream) handleKLineEvent(e *backpackapi.KLineEvent) {
	kline := kLineEventToGlobalKLine(e)
	if kline.Closed {
		s.EmitKLineClosed(kline)
		return
	}

	s.EmitKLine(kline)
}

func (s *Stream) handleBalanceUpdateEvent(e *backpackapi.BalanceUpdateEvent) {
	balance := balanceUpdateEventToGlobalBalance(e)
	s.EmitBalanceUpdate(types.BalanceMap{balance.Currency: balance})
}

// handleOrderUpdateEvent forwards the order update, and the fill it carries when the event is
// an orderFill.
func (s *Stream) handleOrderUpdateEvent(e *backpackapi.OrderUpdateEvent) {
	// emit the trade before the order, which is the order the trade collector expects
	if trade, ok := orderUpdateEventToGlobalTrade(e); ok {
		s.EmitTradeUpdate(trade)
	}

	order, err := orderUpdateEventToGlobalOrder(e)
	if err != nil {
		if orderLogLimiter.Allow() {
			s.logger.WithError(err).Errorf("unable to convert the order update: %+v", e)
		}

		return
	}

	s.EmitOrderUpdate(*order)
}

// Unsubscribe implements types.Unsubscriber.
func (s *Stream) Unsubscribe() {
	streams := make([]string, 0, len(s.Subscriptions))
	for _, subscription := range s.Subscriptions {
		stream, err := s.convertSubscription(subscription)
		if err != nil {
			continue
		}

		streams = append(streams, stream)
	}

	if len(streams) > 0 && s.Conn != nil {
		if err := s.Conn.WriteJSON(backpackapi.NewWebsocketUnsubscribeRequest(streams)); err != nil {
			s.logger.WithError(err).Warn("unsubscribe error")
		}
	}

	if err := s.Resubscribe(func(old []types.Subscription) ([]types.Subscription, error) {
		return []types.Subscription{}, nil
	}); err != nil {
		s.logger.WithError(err).Warn("resubscribe error")
	}
}

// String is used in the logs to tell the public and the private stream apart.
func (s *Stream) String() string {
	mode := "user data"
	if s.PublicOnly {
		mode = "public"
	}

	return "backpack " + mode + " stream"
}

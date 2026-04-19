package hyperliquid

import (
	"context"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/hyperliquid/hyperapi"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	pingInterval = 15 * time.Second
)

// WsSubscription is the Hyperliquid WebSocket subscription object.
// See https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/websocket/subscriptions
type WsSubscription struct {
	Type     string `json:"type"`
	Coin     string `json:"coin,omitempty"`
	Interval string `json:"interval,omitempty"`
	User     string `json:"user,omitempty"`
	Dex      string `json:"dex,omitempty"`
}

// wsSubscribeMsg is the message sent to subscribe or unsubscribe.
type wsSubscribeMsg struct {
	Method       string         `json:"method"`
	Subscription WsSubscription `json:"subscription"`
}

// Stream represents the Hyperliquid websocket stream.
//
//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream

	client   *hyperapi.Client
	exchange *Exchange

	// lastKLines tracks the last received kline per "symbol:interval" key,
	// used to detect candle close by open-time (t) change.
	lastKLinesMu sync.Mutex
	lastKLines   map[string]types.KLine
}

func NewStream(client *hyperapi.Client, ex *Exchange) *Stream {
	s := &Stream{
		client:         client,
		exchange:       ex,
		StandardStream: types.NewStandardStream(),
		lastKLines:     make(map[string]types.KLine),
	}

	s.SetParser(parseWebSocketEvent)
	s.SetDispatcher(s.dispatchEvent)
	s.SetEndpointCreator(s.createEndpoint)
	s.SetPingInterval(pingInterval)
	s.OnConnect(s.handleConnect)

	return s
}

func (s *Stream) createEndpoint(ctx context.Context) (string, error) {
	if hyperapi.TestNet {
		return hyperapi.TestNetWsURL, nil
	}
	return hyperapi.ProductionWsURL, nil
}

// convertSubscription converts a types.Subscription to Hyperliquid WsSubscription.
func convertSubscription(sub types.Subscription, ex *Exchange) (WsSubscription, error) {
	coin, _ := ex.getLocalSymbol(sub.Symbol)
	switch sub.Channel {
	case types.BookChannel, types.BookTickerChannel:
		return WsSubscription{Type: "l2Book", Coin: coin}, nil
	case types.MarketTradeChannel:
		return WsSubscription{Type: "trades", Coin: coin}, nil
	case types.KLineChannel:
		interval, err := toLocalInterval(sub.Options.Interval)
		if err != nil {
			return WsSubscription{}, err
		}
		return WsSubscription{Type: "candle", Coin: coin, Interval: interval}, nil
	default:
		return WsSubscription{}, nil
	}
}

func (s *Stream) handleConnect() {
	conn := s.Conn
	if conn == nil {
		return
	}
	account := s.client.Account()
	// Public subscriptions from user
	subs := s.GetSubscriptions()
	for _, sub := range subs {
		wsSub, err := convertSubscription(sub, s.exchange)
		if err != nil {
			log.WithError(err).Errorf("hyperliquid: convert subscription %+v", sub)
			continue
		}
		if wsSub.Type == "" {
			continue
		}
		msg := wsSubscribeMsg{Method: "subscribe", Subscription: wsSub}
		if err := conn.WriteJSON(msg); err != nil {
			log.WithError(err).Errorf("hyperliquid: send subscribe %+v", wsSub)
			continue
		}
		log.Infof("hyperliquid: subscribed %+v", wsSub)
	}
	// Private channels: subscribe to orderUpdates and userFills when account is set (no separate login on Hyperliquid).
	if !s.PublicOnly && account != "" {
		for _, wsSub := range []WsSubscription{
			{Type: "orderUpdates", User: account},
			{Type: "userFills", User: account},
		} {
			msg := wsSubscribeMsg{Method: "subscribe", Subscription: wsSub}
			if err := conn.WriteJSON(msg); err != nil {
				log.WithError(err).Errorf("hyperliquid: send private subscribe %+v", wsSub)
				continue
			}
			log.Infof("hyperliquid: subscribed private %+v", wsSub)
		}
		s.EmitAuth()
	}
}

func (s *Stream) dispatchEvent(e any) {
	if e == nil {
		return
	}
	switch ev := e.(type) {
	case *SubscriptionResponseEvent:
		// Subscription confirmed; no action needed.
		return
	case *WsBookEvent:
		book := toGlobalOrderBook(ev.Book, s.exchange.IsFutures)
		s.EmitBookSnapshot(*book)
		if bid, okBid := book.BestBid(); okBid {
			if ask, okAsk := book.BestAsk(); okAsk {
				s.EmitBookTickerUpdate(types.BookTicker{
					Symbol:   book.Symbol,
					Buy:      bid.Price,
					BuySize:  bid.Volume,
					Sell:     ask.Price,
					SellSize: ask.Volume,
				})
			}
		}
	case *WsTradesEvent:
		for _, w := range ev.Trades {
			trade := toGlobalMarketTrade(w, s.exchange.IsFutures)
			s.EmitMarketTrade(trade)
		}
	case *WsCandleEvent:
		kline := toGlobalKLine(ev.Candle, s.exchange.IsFutures)
		s.emitCandle(kline)
	case *WsUserFillsEvent:
		if ev.UserFills.IsSnapshot != nil && *ev.UserFills.IsSnapshot {
			// Optional: skip snapshot or merge into state
			return
		}
		for _, f := range ev.UserFills.Fills {
			trade := toGlobalPrivateTrade(f, s.exchange.IsFutures)
			s.EmitTradeUpdate(trade)
		}
	case *WsOrderUpdateEvent:
		order := toGlobalOrderUpdate(ev.OrderUpdate, s.exchange.IsFutures)
		s.EmitOrderUpdate(order)
	case *WsClearinghouseStateEvent:
		// Convert clearinghouse state to positions and emit
		positions := toGlobalFuturesPositions(ev.State)
		if len(positions) > 0 {
			s.EmitFuturesPositionUpdate(positions)
		}
		// Note: Balance updates from clearinghouse state are not emitted here
		// because the MarginSummary doesn't provide per-asset balance breakdown.
		// Use the REST API QueryAccount for detailed balance information.
	}
}

// Unsubscribe sends unsubscribe for all current subscriptions (including private) and clears the list.
func (s *Stream) Unsubscribe() {
	conn := s.Conn
	if conn != nil {
		account := s.client.Account()
		for _, sub := range s.GetSubscriptions() {
			wsSub, err := convertSubscription(sub, s.exchange)
			if err != nil || wsSub.Type == "" {
				continue
			}
			msg := wsSubscribeMsg{Method: "unsubscribe", Subscription: wsSub}
			_ = conn.WriteJSON(msg)
		}
		if !s.PublicOnly && account != "" {
			for _, wsSub := range []WsSubscription{
				{Type: "orderUpdates", User: account},
				{Type: "userFills", User: account},
			} {
				msg := wsSubscribeMsg{Method: "unsubscribe", Subscription: wsSub}
				_ = conn.WriteJSON(msg)
			}
		}
	}
	_ = s.Resubscribe(func(old []types.Subscription) ([]types.Subscription, error) {
		return nil, nil
	})
}

// Connect overrides to use StandardStream.Connect only (no separate kline stream).
func (s *Stream) Connect(ctx context.Context) error {
	return s.StandardStream.Connect(ctx)
}

// Subscribe forwards to StandardStream; handleConnect will send Hyperliquid subscribe messages.
func (s *Stream) Subscribe(channel types.Channel, symbol string, options types.SubscribeOptions) {
	s.StandardStream.Subscribe(channel, symbol, options)
}

// GetSubscriptions returns the current subscriptions (alias for StandardStream.GetSubscriptions).
func (s *Stream) GetSubscriptions() []types.Subscription {
	return s.StandardStream.GetSubscriptions()
}

// emitCandle handles a live candle update from Hyperliquid.
//
// Hyperliquid pushes updates for the current open candle on every trade.
// A candle is considered closed when the next message carries a different
// OpenTime (t field). We detect that transition here and emit KLineClosed
// for the previous candle before emitting the live KLine update.
func (s *Stream) emitCandle(kline types.KLine) {
	key := kline.Symbol + ":" + string(kline.Interval)

	s.lastKLinesMu.Lock()
	prev, hasPrev := s.lastKLines[key]
	s.lastKLines[key] = kline
	s.lastKLinesMu.Unlock()

	// When OpenTime changes, the previous candle is now closed.
	if hasPrev && !prev.StartTime.Time().Equal(kline.StartTime.Time()) {
		closed := prev
		closed.Closed = true
		s.EmitKLineClosed(closed)
	}

	s.EmitKLine(kline)
}

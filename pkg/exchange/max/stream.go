package max

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/depth"
	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream
	types.MarginSettings

	key, secret string

	privateChannels []string

	authEventCallbacks         []func(e maxapi.AuthEvent)
	bookEventCallbacks         []func(e maxapi.BookEvent)
	tradeEventCallbacks        []func(e maxapi.PublicTradeEvent)
	kLineEventCallbacks        []func(e maxapi.KLineEvent)
	errorEventCallbacks        []func(e maxapi.ErrorEvent)
	subscriptionEventCallbacks []func(e maxapi.SubscriptionEvent)

	tradeUpdateEventCallbacks []func(e maxapi.TradeUpdateEvent)

	tradeSnapshotEventCallbacks []func(e maxapi.TradeSnapshotEvent)
	orderUpdateEventCallbacks   []func(e maxapi.OrderUpdateEvent)
	orderSnapshotEventCallbacks []func(e maxapi.OrderSnapshotEvent)
	adRatioEventCallbacks       []func(e maxapi.ADRatioEvent)
	debtEventCallbacks          []func(e maxapi.DebtEvent)

	accountSnapshotEventCallbacks []func(e maxapi.AccountSnapshotEvent)
	accountUpdateEventCallbacks   []func(e maxapi.AccountUpdateEvent)

	fastTradeEnabled bool

	// depthBuffers is used for storing the depth info
	depthBuffers map[string]*depth.Buffer

	debts map[string]maxapi.Debt
}

func NewStream(ex *Exchange, key, secret string) *Stream {
	stream := &Stream{
		StandardStream: types.NewStandardStream(),
		key:            key,
		// pragma: allowlist nextline secret
		secret:       secret,
		depthBuffers: make(map[string]*depth.Buffer),
		debts:        make(map[string]maxapi.Debt, 20),
	}
	stream.SetEndpointCreator(stream.getEndpoint)
	stream.SetParser(maxapi.ParseMessage)
	stream.SetDispatcher(stream.dispatchEvent)
	stream.OnConnect(stream.handleConnect)
	stream.OnDisconnect(stream.handleDisconnect)
	stream.OnDebtEvent(stream.handleDebtEvent)
	stream.OnAuthEvent(func(e maxapi.AuthEvent) {
		log.Infof("max websocket connection authenticated: %+v", e)
		stream.EmitAuth()
	})

	stream.OnKLineEvent(stream.handleKLineEvent)
	stream.OnOrderSnapshotEvent(stream.handleOrderSnapshotEvent)
	stream.OnOrderUpdateEvent(stream.handleOrderUpdateEvent)

	stream.OnTradeUpdateEvent(stream.handleTradeEvent)

	stream.OnAccountSnapshotEvent(stream.handleAccountSnapshotEvent)
	stream.OnAccountUpdateEvent(stream.handleAccountUpdateEvent)
	stream.OnBookEvent(stream.handleBookEvent(ex))
	return stream
}

func (s *Stream) getEndpoint(ctx context.Context) (string, error) {
	url := os.Getenv("MAX_API_WS_URL")
	if url == "" {
		url = maxapi.WebSocketURL
	}
	return url, nil
}

func (s *Stream) SetPrivateChannels(channels []string) {
	// validate channels
	tradeUpdate := 0
	fastTrade := false
	for _, chstr := range channels {
		ch := PrivateChannel(chstr)
		if _, ok := AllPrivateChannels[ch]; !ok {
			log.Errorf("invalid user data stream channel: %s", ch)
		}

		switch ch {
		case PrivateChannelFastTradeUpdate, PrivateChannelTradeUpdate, PrivateChannelMWalletTrade, PrivateChannelMWalletFastTradeUpdate:
			tradeUpdate++
			if ch == PrivateChannelFastTradeUpdate || ch == PrivateChannelMWalletFastTradeUpdate {
				fastTrade = true
			}
		}
	}

	if tradeUpdate > 1 {
		log.Errorf("you can only subscribe to one trade update channel, there are %d trade update channels", tradeUpdate)
	}

	if fastTrade {
		log.Infof("fast trade update is enabled")
	}

	s.fastTradeEnabled = fastTrade
	s.privateChannels = channels
}

func toLocalDepth(depth types.Depth) int {
	if len(depth) > 0 {
		switch depth {
		case types.DepthLevelFull:
			return 50
		case types.DepthLevelMedium:
			return 20
		case types.DepthLevel1:
			return 1
		case types.DepthLevel5:
			return 5
		default:
			return 20
		}
	}

	return 0
}

func (s *Stream) handleConnect() {
	if s.PublicOnly {
		cmd := &maxapi.WebsocketCommand{
			Action: "subscribe",
		}
		for _, sub := range s.Subscriptions {
			depth := toLocalDepth(sub.Options.Depth)

			cmd.Subscriptions = append(cmd.Subscriptions, maxapi.Subscription{
				Channel:    string(sub.Channel),
				Market:     toLocalSymbol(sub.Symbol),
				Depth:      depth,
				Resolution: sub.Options.Interval.String(),
			})
		}

		log.Infof("public subscription commands: %+v", cmd)
		if err := s.Conn.WriteJSON(cmd); err != nil {
			log.WithError(err).Error("failed to send subscription request")
		}

	} else {
		var filters []string

		if len(s.privateChannels) > 0 {
			filters = s.privateChannels
		} else {
			if s.MarginSettings.IsMargin {
				filters = PrivateChannelStrings(defaultMarginPrivateChannels)
			} else {
				filters = PrivateChannelStrings(defaultSpotPrivateChannels)
			}
		}

		log.Debugf("user data websocket channels: %v", filters)

		nonce := time.Now().UnixNano() / int64(time.Millisecond)
		auth := &maxapi.AuthMessage{
			// pragma: allowlist nextline secret
			Action: "auth",
			// pragma: allowlist nextline secret
			APIKey:    s.key,
			Nonce:     nonce,
			Signature: signPayload(strconv.FormatInt(nonce, 10), s.secret),
			ID:        uuid.New().String(),
			Filters:   filters,
		}

		if err := s.Conn.WriteJSON(auth); err != nil {
			log.WithError(err).Error("failed to send auth request")
		}
	}
}

func (s *Stream) handleDisconnect() {
	log.Info("resetting depth snapshots...")
	for _, f := range s.depthBuffers {
		f.Reset()
	}
}

func (s *Stream) handleKLineEvent(e maxapi.KLineEvent) {
	kline := e.KLine.KLine()
	s.EmitKLine(kline)
	if kline.Closed {
		s.EmitKLineClosed(kline)
	}
}

func (s *Stream) handleOrderSnapshotEvent(e maxapi.OrderSnapshotEvent) {
	for _, o := range e.Orders {
		globalOrder, err := convertWebSocketOrderUpdate(o)
		if err != nil {
			log.WithError(err).Error("websocket order snapshot convert error")
			continue
		}

		s.EmitOrderUpdate(*globalOrder)
	}
}

func (s *Stream) handleOrderUpdateEvent(e maxapi.OrderUpdateEvent) {
	for _, o := range e.Orders {
		globalOrder, err := convertWebSocketOrderUpdate(o)
		if err != nil {
			log.WithError(err).Error("websocket order update convert error")
			continue
		}

		s.EmitOrderUpdate(*globalOrder)
	}
}

func (s *Stream) handleTradeEvent(e maxapi.TradeUpdateEvent) {
	for _, tradeUpdate := range e.Trades {
		trade, err := convertWebSocketTrade(tradeUpdate)
		if err != nil {
			log.WithError(err).Error("websocket trade update convert error")
			return
		}

		s.EmitTradeUpdate(*trade)
	}
}

// handleBookEvent returns a callback that will be registered to the websocket stream
// this callback will be called when the websocket stream receives a book event
func (s *Stream) handleBookEvent(ex *Exchange) func(e maxapi.BookEvent) {
	return func(e maxapi.BookEvent) {
		symbol := toGlobalSymbol(e.Market)
		f, ok := s.depthBuffers[symbol]
		if !ok {
			bookDepth := 0
			for _, subscription := range s.Subscriptions {
				if subscription.Channel == types.BookChannel && toLocalSymbol(subscription.Symbol) == e.Market {
					bookDepth = toLocalDepth(subscription.Options.Depth)
					break
				}
			}

			// the default depth of websocket channel is 50, we need to make sure both RESTful API and WS channel use the same depth
			if bookDepth == 0 {
				bookDepth = 50
			}

			f = depth.NewBuffer(func() (types.SliceOrderBook, int64, error) {
				log.Infof("fetching %s depth with depth = %d...", e.Market, bookDepth)
				// the depth of websocket orderbook event is 50 by default, so we use 50 as limit here
				return ex.QueryDepth(context.Background(), e.Market, bookDepth)
			}, 3*time.Second)
			f.SetLogger(logrus.WithFields(logrus.Fields{"exchange": "max", "symbol": symbol, "component": "depthBuffer"}))
			f.OnReady(func(snapshot types.SliceOrderBook, updates []depth.Update) {
				s.EmitBookSnapshot(snapshot)
				for _, u := range updates {
					s.EmitBookUpdate(u.Object)
				}
			})
			f.OnPush(func(update depth.Update) {
				s.EmitBookUpdate(update.Object)
			})
			s.depthBuffers[symbol] = f
		}

		// if we receive orderbook event with both asks and bids are empty, it means we need to rebuild this orderbook
		shouldReset := len(e.Asks) == 0 && len(e.Bids) == 0
		if shouldReset {
			log.Infof("resetting %s orderbook due to both empty asks/bids...", e.Market)
			f.Reset()
			return
		}

		if e.Event == maxapi.BookEventSnapshot {
			if err := f.SetSnapshot(types.SliceOrderBook{
				Symbol:       symbol,
				Time:         e.Time(),
				Bids:         e.Bids,
				Asks:         e.Asks,
				LastUpdateId: e.LastUpdateID,
			}, e.FirstUpdateID, e.LastUpdateID); err != nil {
				log.WithError(err).Errorf("failed to set %s snapshot", e.Market)
			}
		} else {
			if err := f.AddUpdate(types.SliceOrderBook{
				Symbol:       symbol,
				Time:         e.Time(),
				Bids:         e.Bids,
				Asks:         e.Asks,
				LastUpdateId: e.LastUpdateID,
			}, e.FirstUpdateID, e.LastUpdateID); err != nil {
				log.WithError(err).Errorf("found missing %s update event", e.Market)
			}
		}
	}
}

func (s *Stream) String() string {
	ss := "max.Stream"

	if s.PublicOnly {
		ss += " (public only)"
	} else {
		ss += " (user data)"
	}

	if s.MarginSettings.IsMargin {
		ss += " (margin)"
	}

	return ss
}

func (s *Stream) handleDebtEvent(e maxapi.DebtEvent) {
	for _, debt := range e.Debts {
		currency := toGlobalCurrency(debt.Currency)
		s.debts[currency] = debt
	}
}

func (s *Stream) handleAccountSnapshotEvent(e maxapi.AccountSnapshotEvent) {
	snapshot := types.BalanceMap{}
	for _, bm := range e.Balances {
		bal := types.Balance{
			Currency:  toGlobalCurrency(bm.Currency),
			Locked:    bm.Locked,
			Available: bm.Available,
			Borrowed:  fixedpoint.Zero,
			Interest:  fixedpoint.Zero,
		}

		if debt, ok := s.debts[bal.Currency]; ok {
			bal.Borrowed = debt.DebtPrincipal
			bal.Interest = debt.DebtInterest
		}

		snapshot[bal.Currency] = bal
	}

	s.EmitBalanceSnapshot(snapshot)
}

func (s *Stream) handleAccountUpdateEvent(e maxapi.AccountUpdateEvent) {
	snapshot := types.BalanceMap{}
	for _, bm := range e.Balances {
		bal := types.Balance{
			Currency:  toGlobalCurrency(bm.Currency),
			Available: bm.Available,
			Locked:    bm.Locked,
			Borrowed:  fixedpoint.Zero,
			Interest:  fixedpoint.Zero,
		}

		if debt, ok := s.debts[bal.Currency]; ok {
			bal.Borrowed = debt.DebtPrincipal
			bal.Interest = debt.DebtInterest
		}

		snapshot[bal.Currency] = bal
	}

	s.EmitBalanceUpdate(snapshot)
}

func (s *Stream) dispatchEvent(e interface{}) {
	switch e := e.(type) {

	case *maxapi.AuthEvent:
		s.EmitAuthEvent(*e)

	case *maxapi.BookEvent:
		s.EmitBookEvent(*e)

	case *maxapi.PublicTradeEvent:
		s.EmitTradeEvent(*e)

	case *maxapi.KLineEvent:
		s.EmitKLineEvent(*e)

	case *maxapi.ErrorEvent:
		s.EmitErrorEvent(*e)

	case *maxapi.SubscriptionEvent:
		s.EmitSubscriptionEvent(*e)

	case *maxapi.TradeSnapshotEvent:
		s.EmitTradeSnapshotEvent(*e)

	case *maxapi.TradeUpdateEvent:
		s.EmitTradeUpdateEvent(*e)

	case *maxapi.AccountSnapshotEvent:
		s.EmitAccountSnapshotEvent(*e)

	case *maxapi.AccountUpdateEvent:
		s.EmitAccountUpdateEvent(*e)

	case *maxapi.OrderSnapshotEvent:
		s.EmitOrderSnapshotEvent(*e)

	case *maxapi.OrderUpdateEvent:
		s.EmitOrderUpdateEvent(*e)

	case *maxapi.ADRatioEvent:
		s.EmitAdRatioEvent(*e)

	case *maxapi.DebtEvent:
		s.EmitDebtEvent(*e)

	default:
		log.Warnf("unhandled %T event: %+v", e, e)
	}
}

func signPayload(payload string, secret string) string {
	var sig = hmac.New(sha256.New, []byte(secret))
	_, err := sig.Write([]byte(payload))
	if err != nil {
		return ""
	}
	return hex.EncodeToString(sig.Sum(nil))
}

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

	"github.com/c9s/bbgo/pkg/depth"
	max "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream
	types.MarginSettings

	key, secret string

	privateChannels []string

	authEventCallbacks         []func(e max.AuthEvent)
	bookEventCallbacks         []func(e max.BookEvent)
	tradeEventCallbacks        []func(e max.PublicTradeEvent)
	kLineEventCallbacks        []func(e max.KLineEvent)
	errorEventCallbacks        []func(e max.ErrorEvent)
	subscriptionEventCallbacks []func(e max.SubscriptionEvent)

	tradeUpdateEventCallbacks []func(e max.TradeUpdateEvent)

	tradeSnapshotEventCallbacks []func(e max.TradeSnapshotEvent)
	orderUpdateEventCallbacks   []func(e max.OrderUpdateEvent)
	orderSnapshotEventCallbacks []func(e max.OrderSnapshotEvent)
	adRatioEventCallbacks       []func(e max.ADRatioEvent)
	debtEventCallbacks          []func(e max.DebtEvent)

	accountSnapshotEventCallbacks []func(e max.AccountSnapshotEvent)
	accountUpdateEventCallbacks   []func(e max.AccountUpdateEvent)

	fastTradeEnabled bool

	// depthBuffers is used for storing the depth info
	depthBuffers map[string]*depth.Buffer
}

func NewStream(ex *Exchange, key, secret string) *Stream {
	stream := &Stream{
		StandardStream: types.NewStandardStream(),
		key:            key,
		// pragma: allowlist nextline secret
		secret:       secret,
		depthBuffers: make(map[string]*depth.Buffer),
	}
	stream.SetEndpointCreator(stream.getEndpoint)
	stream.SetParser(max.ParseMessage)
	stream.SetDispatcher(stream.dispatchEvent)
	stream.OnConnect(stream.handleConnect)
	stream.OnDisconnect(stream.handleDisconnect)
	stream.OnAuthEvent(func(e max.AuthEvent) {
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
		url = max.WebSocketURL
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

func ToLocalDepth(depth types.Depth) int {
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
		cmd := &max.WebsocketCommand{
			Action: "subscribe",
		}
		for _, sub := range s.Subscriptions {
			depth := ToLocalDepth(sub.Options.Depth)

			cmd.Subscriptions = append(cmd.Subscriptions, max.Subscription{
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
		auth := &max.AuthMessage{
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
	log.Debugf("resetting depth snapshots...")
	for _, f := range s.depthBuffers {
		f.Reset()
	}
}

func (s *Stream) handleKLineEvent(e max.KLineEvent) {
	kline := e.KLine.KLine()
	s.EmitKLine(kline)
	if kline.Closed {
		s.EmitKLineClosed(kline)
	}
}

func (s *Stream) handleOrderSnapshotEvent(e max.OrderSnapshotEvent) {
	for _, o := range e.Orders {
		globalOrder, err := convertWebSocketOrderUpdate(o)
		if err != nil {
			log.WithError(err).Error("websocket order snapshot convert error")
			continue
		}

		s.EmitOrderUpdate(*globalOrder)
	}
}

func (s *Stream) handleOrderUpdateEvent(e max.OrderUpdateEvent) {
	for _, o := range e.Orders {
		globalOrder, err := convertWebSocketOrderUpdate(o)
		if err != nil {
			log.WithError(err).Error("websocket order update convert error")
			continue
		}

		s.EmitOrderUpdate(*globalOrder)
	}
}

func (s *Stream) handleTradeEvent(e max.TradeUpdateEvent) {
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
func (s *Stream) handleBookEvent(ex *Exchange) func(e max.BookEvent) {
	return func(e max.BookEvent) {
		symbol := toGlobalSymbol(e.Market)
		f, ok := s.depthBuffers[symbol]
		if !ok {
			bookDepth := 0
			for _, subscription := range s.Subscriptions {
				if subscription.Channel == types.BookChannel && toLocalSymbol(subscription.Symbol) == e.Market {
					bookDepth = ToLocalDepth(subscription.Options.Depth)
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
			f.Reset()
			return
		}

		if e.Event == max.BookEventSnapshot {
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

func (s *Stream) handleAccountSnapshotEvent(e max.AccountSnapshotEvent) {
	snapshot := map[string]types.Balance{}
	for _, bm := range e.Balances {
		balance, err := bm.Balance()
		if err != nil {
			continue
		}

		snapshot[balance.Currency] = *balance
	}

	s.EmitBalanceSnapshot(snapshot)
}

func (s *Stream) handleAccountUpdateEvent(e max.AccountUpdateEvent) {
	snapshot := map[string]types.Balance{}
	for _, bm := range e.Balances {
		balance, err := bm.Balance()
		if err != nil {
			continue
		}

		snapshot[toGlobalCurrency(balance.Currency)] = *balance
	}

	s.EmitBalanceUpdate(snapshot)
}

func (s *Stream) dispatchEvent(e interface{}) {
	switch e := e.(type) {

	case *max.AuthEvent:
		s.EmitAuthEvent(*e)

	case *max.BookEvent:
		s.EmitBookEvent(*e)

	case *max.PublicTradeEvent:
		s.EmitTradeEvent(*e)

	case *max.KLineEvent:
		s.EmitKLineEvent(*e)

	case *max.ErrorEvent:
		s.EmitErrorEvent(*e)

	case *max.SubscriptionEvent:
		s.EmitSubscriptionEvent(*e)

	case *max.TradeSnapshotEvent:
		s.EmitTradeSnapshotEvent(*e)

	case *max.TradeUpdateEvent:
		s.EmitTradeUpdateEvent(*e)

	case *max.AccountSnapshotEvent:
		s.EmitAccountSnapshotEvent(*e)

	case *max.AccountUpdateEvent:
		s.EmitAccountUpdateEvent(*e)

	case *max.OrderSnapshotEvent:
		s.EmitOrderSnapshotEvent(*e)

	case *max.OrderUpdateEvent:
		s.EmitOrderUpdateEvent(*e)

	case *max.ADRatioEvent:
		s.EmitAdRatioEvent(*e)

	case *max.DebtEvent:
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

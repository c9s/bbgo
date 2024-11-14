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

	tradeUpdateEventCallbacks   []func(e max.TradeUpdateEvent)
	tradeSnapshotEventCallbacks []func(e max.TradeSnapshotEvent)
	orderUpdateEventCallbacks   []func(e max.OrderUpdateEvent)
	orderSnapshotEventCallbacks []func(e max.OrderSnapshotEvent)
	adRatioEventCallbacks       []func(e max.ADRatioEvent)
	debtEventCallbacks          []func(e max.DebtEvent)

	accountSnapshotEventCallbacks []func(e max.AccountSnapshotEvent)
	accountUpdateEventCallbacks   []func(e max.AccountUpdateEvent)

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
	s.privateChannels = channels
}

func (s *Stream) handleConnect() {
	if s.PublicOnly {
		cmd := &max.WebsocketCommand{
			Action: "subscribe",
		}
		for _, sub := range s.Subscriptions {
			var depth int

			if len(sub.Options.Depth) > 0 {
				switch sub.Options.Depth {
				case types.DepthLevelFull:
					depth = 50

				case types.DepthLevelMedium:
					depth = 20

				case types.DepthLevel1:
					depth = 1

				case types.DepthLevel5:
					depth = 5

				default:
					depth = 20

				}
			}

			cmd.Subscriptions = append(cmd.Subscriptions, max.Subscription{
				Channel:    string(sub.Channel),
				Market:     toLocalSymbol(sub.Symbol),
				Depth:      depth,
				Resolution: sub.Options.Interval.String(),
			})
		}

		if err := s.Conn.WriteJSON(cmd); err != nil {
			log.WithError(err).Error("failed to send subscription request")
		}

	} else {
		var filters []string

		if len(s.privateChannels) > 0 {
			// TODO: maybe check the valid private channels
			filters = s.privateChannels
		} else if s.MarginSettings.IsMargin {
			filters = []string{
				"mwallet_order",
				"mwallet_trade",
				"mwallet_account",
				"ad_ratio",
				"borrowing",
			}
		}

		log.Debugf("user data websocket filters: %v", filters)

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

func (s *Stream) handleBookEvent(ex *Exchange) func(e max.BookEvent) {
	return func(e max.BookEvent) {
		symbol := toGlobalSymbol(e.Market)
		f, ok := s.depthBuffers[symbol]
		if !ok {
			f = depth.NewBuffer(func() (types.SliceOrderBook, int64, error) {
				log.Infof("fetching %s depth...", e.Market)
				// the depth of websocket orderbook event is 50 by default, so we use 50 as limit here
				return ex.QueryDepth(context.Background(), e.Market, 50)
			})
			f.SetBufferingPeriod(time.Second)
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

		if err := f.AddUpdate(types.SliceOrderBook{
			Symbol: symbol,
			Time:   e.Time(),
			Bids:   e.Bids,
			Asks:   e.Asks,
		}, e.FirstUpdateID, e.LastUpdateID); err != nil {
			log.WithError(err).Errorf("found missing %s update event", e.Market)
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

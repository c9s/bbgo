package max

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	max "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream
	types.MarginSettings

	key, secret string

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
}

func NewStream(key, secret string) *Stream {
	stream := &Stream{
		StandardStream: types.NewStandardStream(),
		key:            key,
		// pragma: allowlist nextline secret
		secret: secret,
	}
	stream.SetEndpointCreator(stream.getEndpoint)
	stream.SetParser(max.ParseMessage)
	stream.SetDispatcher(stream.dispatchEvent)
	stream.OnConnect(stream.handleConnect)
	stream.OnAuthEvent(func(e max.AuthEvent) {
		log.Infof("max websocket connection authenticated: %+v", e)
	})
	stream.OnKLineEvent(stream.handleKLineEvent)
	stream.OnOrderSnapshotEvent(stream.handleOrderSnapshotEvent)
	stream.OnOrderUpdateEvent(stream.handleOrderUpdateEvent)
	stream.OnTradeUpdateEvent(stream.handleTradeEvent)
	stream.OnBookEvent(stream.handleBookEvent)
	stream.OnAccountSnapshotEvent(stream.handleAccountSnapshotEvent)
	stream.OnAccountUpdateEvent(stream.handleAccountUpdateEvent)
	return stream
}

func (s *Stream) getEndpoint(ctx context.Context) (string, error) {
	url := os.Getenv("MAX_API_WS_URL")
	if url == "" {
		url = max.WebSocketURL
	}
	return url, nil
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
					depth = 0

				case types.DepthLevelMedium:
					depth = 20

				case types.DepthLevel5:
					depth = 5

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
		if s.MarginSettings.IsMargin {
			filters = []string{
				"mwallet_order",
				"mwallet_trade",
				"mwallet_account",
				"ad_ratio",
				"borrowing",
			}
		}

		nonce := time.Now().UnixNano() / int64(time.Millisecond)
		auth := &max.AuthMessage{
			// pragma: allowlist nextline secret
			Action: "auth",
			// pragma: allowlist nextline secret
			APIKey:    s.key,
			Nonce:     nonce,
			Signature: signPayload(fmt.Sprintf("%d", nonce), s.secret),
			ID:        uuid.New().String(),
			Filters:   filters,
		}

		if err := s.Conn.WriteJSON(auth); err != nil {
			log.WithError(err).Error("failed to send auth request")
		}
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

func (s *Stream) handleBookEvent(e max.BookEvent) {
	newBook, err := e.OrderBook()
	if err != nil {
		log.WithError(err).Error("book convert error")
		return
	}

	newBook.Symbol = toGlobalSymbol(e.Market)
	newBook.Time = e.Time()

	switch e.Event {
	case "snapshot":
		s.EmitBookSnapshot(newBook)
	case "update":
		s.EmitBookUpdate(newBook)
	}
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

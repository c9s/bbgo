package max

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"

	max "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream

	key, secret string

	bookEventCallbacks         []func(e max.BookEvent)
	tradeEventCallbacks        []func(e max.PublicTradeEvent)
	kLineEventCallbacks        []func(e max.KLineEvent)
	errorEventCallbacks        []func(e max.ErrorEvent)
	subscriptionEventCallbacks []func(e max.SubscriptionEvent)

	tradeUpdateEventCallbacks   []func(e max.TradeUpdateEvent)
	tradeSnapshotEventCallbacks []func(e max.TradeSnapshotEvent)
	orderUpdateEventCallbacks   []func(e max.OrderUpdateEvent)
	orderSnapshotEventCallbacks []func(e max.OrderSnapshotEvent)

	accountSnapshotEventCallbacks []func(e max.AccountSnapshotEvent)
	accountUpdateEventCallbacks   []func(e max.AccountUpdateEvent)
}

func NewStream(key, secret string) *Stream {
	stream := &Stream{
		StandardStream: types.NewStandardStream(),
		key:            key,
		secret:         secret,
	}
	stream.SetEndpointCreator(stream.getEndpoint)
	stream.SetParser(max.ParseMessage)
	stream.SetDispatcher(stream.dispatchEvent)

	stream.OnConnect(stream.handleConnect)
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
			var err error
			var depth int

			if len(sub.Options.Depth) > 0 {
				depth, err = strconv.Atoi(sub.Options.Depth)
				if err != nil {
					log.WithError(err).Errorf("depth parse error, given %v", sub.Options.Depth)
					continue
				}
			}

			cmd.Subscriptions = append(cmd.Subscriptions, max.Subscription{
				Channel:    string(sub.Channel),
				Market:     toLocalSymbol(sub.Symbol),
				Depth:      depth,
				Resolution: sub.Options.Interval,
			})
		}

		s.Conn.WriteJSON(cmd)
	} else {
		nonce := time.Now().UnixNano() / int64(time.Millisecond)
		auth := &max.AuthMessage{
			Action:    "auth",
			APIKey:    s.key,
			Nonce:     nonce,
			Signature: signPayload(fmt.Sprintf("%d", nonce), s.secret),
			ID:        uuid.New().String(),
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
		globalOrder, err := toGlobalOrderUpdate(o)
		if err != nil {
			log.WithError(err).Error("websocket order snapshot convert error")
			continue
		}

		s.EmitOrderUpdate(*globalOrder)
	}
}

func (s *Stream) handleOrderUpdateEvent(e max.OrderUpdateEvent) {
	for _, o := range e.Orders {
		globalOrder, err := toGlobalOrderUpdate(o)
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

func convertWebSocketTrade(t max.TradeUpdate) (*types.Trade, error) {
	// skip trade ID that is the same. however this should not happen
	var side = toGlobalSideType(t.Side)

	// trade time
	mts := time.Unix(0, t.Timestamp*int64(time.Millisecond))

	price, err := strconv.ParseFloat(t.Price, 64)
	if err != nil {
		return nil, err
	}

	quantity, err := strconv.ParseFloat(t.Volume, 64)
	if err != nil {
		return nil, err
	}

	quoteQuantity := price * quantity

	fee, err := strconv.ParseFloat(t.Fee, 64)
	if err != nil {
		return nil, err
	}

	return &types.Trade{
		ID:            t.ID,
		OrderID:       t.OrderID,
		Symbol:        toGlobalSymbol(t.Market),
		Exchange:      types.ExchangeMax,
		Price:         price,
		Quantity:      quantity,
		Side:          side,
		IsBuyer:       side == types.SideTypeBuy,
		IsMaker:       t.Maker,
		Fee:           fee,
		FeeCurrency:   toGlobalCurrency(t.FeeCurrency),
		QuoteQuantity: quoteQuantity,
		Time:          types.Time(mts),
	}, nil
}

func toGlobalOrderUpdate(u max.OrderUpdate) (*types.Order, error) {
	executedVolume, err := fixedpoint.NewFromString(u.ExecutedVolume)
	if err != nil {
		return nil, err
	}

	remainingVolume, err := fixedpoint.NewFromString(u.RemainingVolume)
	if err != nil {
		return nil, err
	}

	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: u.ClientOID,
			Symbol:        toGlobalSymbol(u.Market),
			Side:          toGlobalSideType(u.Side),
			Type:          toGlobalOrderType(u.OrderType),
			Quantity:      util.MustParseFloat(u.Volume),
			Price:         util.MustParseFloat(u.Price),
			StopPrice:     util.MustParseFloat(u.StopPrice),
			TimeInForce:   "GTC", // MAX only supports GTC
			GroupID:       u.GroupID,
		},
		Exchange:         types.ExchangeMax,
		OrderID:          u.ID,
		Status:           toGlobalOrderStatus(u.State, executedVolume, remainingVolume),
		ExecutedQuantity: executedVolume.Float64(),
		CreationTime:     types.Time(time.Unix(0, u.CreatedAtMs*int64(time.Millisecond))),
	}, nil
}

func (s *Stream) dispatchEvent(e interface{}) {
	switch e := e.(type) {

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

	default:
		log.Errorf("unsupported %T event: %+v", e, e)
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

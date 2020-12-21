package max

import (
	"context"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	max "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

var logger = log.WithField("exchange", "max")

type Stream struct {
	types.StandardStream

	websocketService *max.WebSocketService

	publicOnly bool
}

func NewStream(key, secret string) *Stream {
	wss := max.NewWebSocketService(max.WebSocketURL, key, secret)

	stream := &Stream{
		websocketService: wss,
	}

	wss.OnConnect(func(conn *websocket.Conn) {
		if key == "" || secret == "" {
			log.Warn("MAX API key or secret is empty, will not send authentication command")
		} else {
			if err := wss.Auth(); err != nil {
				wss.EmitError(err)
				logger.WithError(err).Error("failed to send auth request")
			}
		}
	})

	wss.OnMessage(func(message []byte) {
		logger.Debugf("M: %s", message)
	})

	wss.OnKLineEvent(func(e max.KLineEvent) {
		kline := e.KLine.KLine()
		stream.EmitKLine(kline)
		if kline.Closed {
			stream.EmitKLineClosed(kline)
		}
	})

	wss.OnOrderSnapshotEvent(func(e max.OrderSnapshotEvent) {
		for _, o := range e.Orders {
			globalOrder, err := toGlobalOrderUpdate(o)
			if err != nil {
				log.WithError(err).Error("websocket order snapshot convert error")
				continue
			}

			stream.EmitOrderUpdate(*globalOrder)
		}
	})

	wss.OnOrderUpdateEvent(func(e max.OrderUpdateEvent) {
		for _, o := range e.Orders {
			globalOrder, err := toGlobalOrderUpdate(o)
			if err != nil {
				log.WithError(err).Error("websocket order update convert error")
				continue
			}

			stream.EmitOrderUpdate(*globalOrder)
		}
	})

	wss.OnTradeUpdateEvent(func(e max.TradeUpdateEvent) {
		for _, tradeUpdate := range e.Trades {
			trade, err := convertWebSocketTrade(tradeUpdate)
			if err != nil {
				log.WithError(err).Error("websocket trade update convert error")
				return
			}

			stream.EmitTradeUpdate(*trade)
		}
	})

	wss.OnBookEvent(func(e max.BookEvent) {
		newBook, err := e.OrderBook()
		if err != nil {
			logger.WithError(err).Error("book convert error")
			return
		}

		newBook.Symbol = toGlobalSymbol(e.Market)

		switch e.Event {
		case "snapshot":
			stream.EmitBookSnapshot(newBook)
		case "update":
			stream.EmitBookUpdate(newBook)
		}
	})

	wss.OnConnect(func(conn *websocket.Conn) {
		stream.EmitConnect()
	})

	wss.OnAccountSnapshotEvent(func(e max.AccountSnapshotEvent) {
		snapshot := map[string]types.Balance{}
		for _, bm := range e.Balances {
			balance, err := bm.Balance()
			if err != nil {
				continue
			}

			snapshot[toGlobalCurrency(balance.Currency)] = *balance
		}

		stream.EmitBalanceSnapshot(snapshot)
	})

	wss.OnAccountUpdateEvent(func(e max.AccountUpdateEvent) {
		snapshot := map[string]types.Balance{}
		for _, bm := range e.Balances {
			balance, err := bm.Balance()
			if err != nil {
				continue
			}

			snapshot[toGlobalCurrency(balance.Currency)] = *balance
		}

		stream.EmitBalanceUpdate(snapshot)
	})

	return stream
}

func (s *Stream) SetPublicOnly() {
	s.publicOnly = true
}

func (s *Stream) Subscribe(channel types.Channel, symbol string, options types.SubscribeOptions) {
	s.websocketService.Subscribe(string(channel), toLocalSymbol(symbol))
}

func (s *Stream) Connect(ctx context.Context) error {
	return s.websocketService.Connect(ctx)
}

func (s *Stream) Close() error {
	return s.websocketService.Close()
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
		ID:            int64(t.ID),
		OrderID:       t.OrderID,
		Symbol:        toGlobalSymbol(t.Market),
		Exchange:      "max",
		Price:         price,
		Quantity:      quantity,
		Side:          side,
		IsBuyer:       side == "bid",
		IsMaker:       t.Maker,
		Fee:           fee,
		FeeCurrency:   toGlobalCurrency(t.FeeCurrency),
		QuoteQuantity: quoteQuantity,
		Time:          mts,
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
		},
		Exchange:         "max",
		OrderID:          u.ID,
		Status:           toGlobalOrderStatus(u.State, executedVolume, remainingVolume),
		ExecutedQuantity: executedVolume.Float64(),
		CreationTime:     time.Unix(0, u.CreatedAtMs*int64(time.Millisecond)),
	}, nil
}

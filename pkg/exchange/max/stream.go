package max

import (
	"context"
	"strconv"
	"time"

	max "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/types"
)

var logger = log.WithField("exchange", "max")

type Stream struct {
	types.StandardStream

	websocketService *max.WebSocketService
}

func NewStream(key, secret string) *Stream {
	wss := max.NewWebSocketService(max.WebSocketURL, key, secret)

	stream := &Stream{
		websocketService: wss,
	}

	wss.OnMessage(func(message []byte) {
		logger.Infof("M: %s", message)
	})

	// wss.OnTradeEvent(func(e max.PublicTradeEvent) { })

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
		newbook, err := e.OrderBook()
		if err != nil {
			logger.WithError(err).Error("book convert error")
			return
		}

		newbook.Symbol = toGlobalSymbol(e.Market)

		switch e.Event {
		case "snapshot":
			stream.EmitBookSnapshot(newbook)
		case "update":
			stream.EmitBookUpdate(newbook)
		}
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
		Price:         price,
		Symbol:        toGlobalSymbol(t.Market),
		Exchange:      "max",
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

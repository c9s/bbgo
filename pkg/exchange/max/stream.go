package max

import (
	"context"

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

	wss.OnBookEvent(func(e max.BookEvent) {
		newbook, err := e.OrderBook()
		if err != nil {
			logger.WithError(err).Error("book convert error")
			return
		}

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
	s.websocketService.Subscribe(string(channel), symbol)
}

func (s *Stream) Connect(ctx context.Context) error {
	return s.websocketService.Connect(ctx)
}

func (s *Stream) Close() error {
	return s.websocketService.Close()
}

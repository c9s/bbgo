package max

import (
	"context"

	log "github.com/sirupsen/logrus"

	max "github.com/c9s/bbgo/pkg/bbgo/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/bbgo/types"
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

	return stream
}

func (s *Stream) Connect(ctx context.Context) error {
	return nil
}

func (s *Stream) Close() error {
	return s.websocketService.Close()
}

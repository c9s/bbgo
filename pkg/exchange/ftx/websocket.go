package ftx

import (
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

type WebsocketService struct {
	*service.WebsocketClientBase

	key    string
	secret string

	subscriptions []SubscribeRequest
}

const endpoint = "wss://ftx.com/ws/"

func NewWebsocketService(key string, secret string) *WebsocketService {
	s := &WebsocketService{
		WebsocketClientBase: service.NewWebsocketClientBase(endpoint, 3*time.Second),
		key:                 key,
		secret:              secret,
	}
	s.OnConnected(func(_ *websocket.Conn) {
		if err := s.sendSubscriptions(); err != nil {
			s.EmitError(err)
		}
	})
	return s
}

func (w *WebsocketService) Subscribe(channel types.Channel, symbol string) error {
	r := SubscribeRequest{
		Operation: subscribe,
	}
	if channel != types.BookChannel {
		return fmt.Errorf("unsupported channel %+v", channel)
	}
	r.Channel = orderbook
	r.Market = strings.ToUpper(strings.TrimSpace(symbol))

	w.subscriptions = append(w.subscriptions, r)
	return nil
}

var errSubscriptionFailed = fmt.Errorf("failed to subscribe")

func (w *WebsocketService) sendSubscriptions() error {
	conn := w.Conn()
	for _, s := range w.subscriptions {
		if err := conn.WriteJSON(s); err != nil {
			return fmt.Errorf("can't send subscription request %+v: %w", s, errSubscriptionFailed)
		}
	}
	return nil
}

func (w *WebsocketService) Close() error {
	return w.Conn().Close()
}

package ftx

import (
	"fmt"
	"time"

	"github.com/gorilla/websocket"

	"github.com/c9s/bbgo/pkg/service"
)

type WebsocketService struct {
	*service.WebsocketClientBase

	key    string
	secret string

	subscriptions []websocketRequest
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

func (w *WebsocketService) Subscribe(request websocketRequest) {
	w.subscriptions = append(w.subscriptions, request)
}

var errSubscriptionFailed = fmt.Errorf("failed to subscribe")

func (w *WebsocketService) sendSubscriptions() error {
	conn := w.Conn()
	for _, s := range w.subscriptions {
		logger.Infof("s: %+v", s)
		if err := conn.WriteJSON(s); err != nil {
			return fmt.Errorf("can't send subscription request %+v: %w", s, errSubscriptionFailed)
		}
	}
	return nil
}

// After closing the websocket connection, you have to subscribe all events again
func (w *WebsocketService) Close() error {
	w.subscriptions = nil
	if conn := w.Conn(); conn != nil {
		return conn.Close()
	}
	return nil
}

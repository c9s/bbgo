package ftx

import (
	"time"

	"github.com/c9s/bbgo/pkg/service"
)

type WebsocketService struct {
	*service.WebsocketClientBase

	key    string
	secret string
}

const endpoint = "wss://ftx.com/ws/"

func NewWebsocketService(key string, secret string) *WebsocketService {
	s := &WebsocketService{
		WebsocketClientBase: service.NewWebsocketClientBase(endpoint, 3*time.Second),
		key:                 key,
		secret:              secret,
	}
	return s
}

func (w *WebsocketService) Close() error {
	return w.Conn().Close()
}

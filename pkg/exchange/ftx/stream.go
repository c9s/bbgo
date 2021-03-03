package ftx

import (
	"context"
	"sync/atomic"

	"github.com/c9s/bbgo/pkg/types"
)

type Stream struct {
	*types.StandardStream

	wsService *WebsocketService

	// publicOnly must be accessed atomically
	publicOnly int32
}

func NewStream(key, secret string) *Stream {
	wss := NewWebsocketService(key, secret)
	s := &Stream{
		StandardStream: &types.StandardStream{},
		wsService:      wss,
	}

	wss.OnMessage(messageHandler{StandardStream: s.StandardStream}.handleMessage)
	return s
}

func (s *Stream) Connect(ctx context.Context) error {
	return s.wsService.Connect(ctx)
}

func (s *Stream) SetPublicOnly() {
	atomic.StoreInt32(&s.publicOnly, 1)
}

func (s *Stream) Subscribe(channel types.Channel, symbol string, _ types.SubscribeOptions) {
	if err := s.wsService.Subscribe(channel, symbol); err != nil {
		logger.WithError(err).Errorf("subscribe failed, should never happen")
	}
}
func (s *Stream) Close() error {
	return s.wsService.Close()
}

package mexc

import (
	"context"

	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -types Stream -interface
type Stream struct {
	types.StandardStream
}

func NewStream(ex *Exchange) *Stream {
	stream := &Stream {
		StandardStream: types.NewStandardStream(),
	}
	return stream
}

func (s *Stream) handleDisconnect() {
}

func (s *Stream) handleConnect() {
}

func (s *Stream) Subscribe(channel types.Channel, symbol string, options types.SubscribeOptions) {
}

func (s *Stream) SetPublicOnly() {
}

func (s *Stream) Connect(ctx context.Context) error {
	return nil
}

func (s *Stream) Close() error {
	return nil
}
var _ types.Stream = &Stream{}

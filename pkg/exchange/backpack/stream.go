package backpack

import (
	"context"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/types"
)

// ErrStreamNotImplemented is returned by Stream.Connect until the websocket stream is
// implemented.
var ErrStreamNotImplemented = errors.New("backpack: websocket stream is not implemented yet")

// Stream is a placeholder for the Backpack websocket stream.
//
// types.Exchange requires NewStream to return a usable types.Stream, and
// bbgo.NewExchangeSession calls it and binds a connectivity watcher to it as soon as a session
// is created, so it has to be constructible. Connect fails loudly rather than blocking, so
// that a session which actually needs market or user data reports the gap instead of hanging.
type Stream struct {
	types.StandardStream
}

func NewStream() *Stream {
	return &Stream{
		StandardStream: types.NewStandardStream(),
	}
}

func (s *Stream) Connect(ctx context.Context) error {
	return ErrStreamNotImplemented
}

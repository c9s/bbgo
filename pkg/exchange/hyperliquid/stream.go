package hyperliquid

import (
	"github.com/c9s/bbgo/pkg/types"
)

// Stream represents the Hyperliquid websocket stream.
//
//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream
}

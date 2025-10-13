package hyperliquid

import (
	"github.com/c9s/bbgo/pkg/exchange/hyperliquid/hyperapi"
	"github.com/c9s/bbgo/pkg/types"
)

// Stream represents the Hyperliquid websocket stream.
//
//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream

	client   *hyperapi.Client
	exchange *Exchange
}

func NewStream(client *hyperapi.Client, ex *Exchange) *Stream {

	return &Stream{
		client:   client,
		exchange: ex,
	}
}

package bitfinex

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/depth"
	bfxapi "github.com/c9s/bbgo/pkg/exchange/bitfinex/bfxapi"
	"github.com/c9s/bbgo/pkg/types"
)

// Stream represents the Bitfinex websocket stream.
//
//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream

	depthBuffers map[string]*depth.Buffer

	tickerEventCallbacks      []func(e *bfxapi.TickerEvent)
	bookEventCallbacks        []func(e *bfxapi.BookEvent)
	candleEventCallbacks      []func(e *bfxapi.CandleEvent)
	statusEventCallbacks      []func(e *bfxapi.StatusEvent)
	marketTradeEventCallbacks []func(e *bfxapi.MarketTradeEvent)

	parser *bfxapi.Parser
}

// NewStream creates a new Bitfinex Stream.
func NewStream() *Stream {
	stream := &Stream{
		StandardStream: types.NewStandardStream(),
		depthBuffers:   make(map[string]*depth.Buffer),

		parser: bfxapi.NewParser(),
	}
	stream.SetParser(stream.parser.Parse)
	stream.SetDispatcher(stream.dispatchEvent)
	stream.SetEndpointCreator(stream.getEndpoint)
	return stream
}

// getEndpoint returns the websocket endpoint URL.
func (s *Stream) getEndpoint(ctx context.Context) (string, error) {
	url := os.Getenv("BITFINEX_API_WS_URL")
	if url == "" {
		if s.PublicOnly {
			url = "wss://api-pub.bitfinex.com/ws/2"
		} else {
			url = "wss://api.bitfinex.com/ws/2"
		}
	}
	return url, nil
}

// dispatchEvent dispatches parsed events to corresponding callbacks.
func (s *Stream) dispatchEvent(e interface{}) {
	switch evt := e.(type) {
	default:
		logrus.Warnf("unhandled %T event: %+v", evt, evt)
	}
}

package bbgo

import (
	"github.com/c9s/bbgo/pkg/types"
)

// NewMarketTradeStream creates an isolated new market trade stream for the given symbol.
func NewMarketTradeStream(session *ExchangeSession, symbol string) types.Stream {
	stream := session.Exchange.NewStream()
	stream.SetPublicOnly()
	stream.Subscribe(types.MarketTradeChannel, symbol, types.SubscribeOptions{})
	return stream
}

// NewBookStream creates an isolated new order book stream for the given symbol with the given options.
// The default options are DepthLevelFull and SpeedLow.
func NewBookStream(session *ExchangeSession, symbol string, va ...types.SubscribeOptions) types.Stream {
	// the default opts
	opts := types.SubscribeOptions{
		Depth: types.DepthLevelFull,
		Speed: types.SpeedLow,
	}

	if len(va) > 0 {
		opts = va[0]
	}

	stream := session.Exchange.NewStream()
	stream.SetPublicOnly()
	stream.Subscribe(types.BookChannel, symbol, opts)
	return stream
}

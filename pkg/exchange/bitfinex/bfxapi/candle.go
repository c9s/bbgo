package bfxapi

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// Candle represents a single candlestick returned by Bitfinex API.
type Candle struct {
	Time types.MillisecondTimestamp // millisecond timestamp

	Open   fixedpoint.Value // open price
	Close  fixedpoint.Value // close price
	High   fixedpoint.Value // high price
	Low    fixedpoint.Value // low price
	Volume fixedpoint.Value // volume
}

// UnmarshalJSON parses a JSON array into a Candle struct.
func (c *Candle) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, c, 0)
}

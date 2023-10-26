package pattern

import "github.com/c9s/bbgo/pkg/fixedpoint"

// OrderSide represents the side of an order: Buy (long) or Sell (short).
type Direction int

const (
	MaxNumOfPattern           = 5_000
	Bullish         Direction = iota + 1
	Bearish
)

var (
	Neutral   = .0
	Bull      = 1.0
	Bear      = -1.0
	threshold = fixedpoint.NewFromFloat(0.1)
	limit     = fixedpoint.NewFromFloat(0.2)
)

func n(n float64) fixedpoint.Value {
	return fixedpoint.NewFromFloat(float64(n))
}

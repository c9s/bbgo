package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// TMAStream is the Triangular Moving Average.
//
// Refer: Triangular Moving Average
// Refer URL: https://ja.wikipedia.org/wiki/移動平均
//
// The Triangular Moving Average is a double-smoothed simple moving average:
//
//	w   = (window + 1) / 2
//	TMA = SMA(SMA(price, w), w)
type TMAStream struct {
	// the outer SMA
	*SMAStream

	// Inner is the inner SMA of the price source
	Inner *SMAStream
}

// TMA creates the triangular moving average stream
func TMA(source types.Float64Source, window int) *TMAStream {
	checkWindow(window)

	w := (window + 1) / 2
	inner := SMA(source, w)
	return &TMAStream{
		SMAStream: SMA(inner, w),
		Inner:     inner,
	}
}

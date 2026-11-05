package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// WWMAStream is the Welles Wilder's Moving Average.
//
// Refer: Welles Wilder's Moving Average
// Refer URL: http://fxcorporate.com/help/MS/NOTFIFO/i_WMA.html
//
// Calculation:
//
//	WWMA[0] = price[0]
//	WWMA[i] = WWMA[i-1] + (price[i] - WWMA[i-1]) / window
//
// This is mathematically identical to the non-adjusted RMA, so it is composed from RMA2.
type WWMAStream struct {
	*RMAStream
}

// WWMA creates the Welles Wilder's moving average stream
func WWMA(source types.Float64Source, window int) *WWMAStream {
	return &WWMAStream{
		RMAStream: RMA2(source, window, false),
	}
}

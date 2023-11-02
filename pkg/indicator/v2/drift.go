package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: https://tradingview.com/script/aDymGrFx-Drift-Study-Inspired-by-Monte-Carlo-Simulations-with-BM-KL/
// Brownian Motion's drift factor
// could be used in Monte Carlo Simulations
//
// In the context of Brownian motion, drift can be measured by calculating the simple moving average (SMA) of the logarithm
// of the price changes of a security over a specified period of time. This SMA can be used to identify the long-term trend
// or bias in the random movement of the security's price. A security with a positive drift is said to be trending upwards,
// while a security with a negative drift is said to be trending downwards. Drift can be used by traders to identify potential
// entry and exit points for trades, or to confirm other technical analysis signals.
// It is typically used in conjunction with other indicators to provide a more comprehensive view of the security's price.
type DriftStream struct {
	*types.Float64Series
	sma    *SMAStream
	window int
}

func Drift(source KLineSubscription, window int) *DriftStream {
	var (
		diffClose = DiffClose(source)
		s         = &DriftStream{
			Float64Series: types.NewFloat64Series(),
			sma:           SMA(diffClose, window),
			window:        window,
		}
	)
	source.AddSubscriber(func(v types.KLine) {
		var drift float64

		if source.Length() > s.window {
			var stdev = diffClose.Stdev(s.window)
			drift = s.sma.Last(0) - stdev*stdev*0.5
		}

		s.PushAndEmit(drift)
	})

	return s
}

func (s *DriftStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfMA)
}

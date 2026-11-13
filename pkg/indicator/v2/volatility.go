package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// VolatilityStream is the population standard deviation of the close prices
// over the last `window` klines.
//
// It is the StdDev of the close prices, except that no value is emitted until
// `window` klines have been received (StdDev emits a partial-window value from
// the first kline on).
//
// Note: this is a kline-based price dispersion, unlike RealizedVolatility, which
// is an EWMA of squared trade log-returns.
type VolatilityStream struct {
	*types.Float64Series

	stddev *StdDevStream
	window int
	count  int
}

func Volatility(source KLineSubscription, window int) *VolatilityStream {
	checkWindow(window)

	s := &VolatilityStream{
		Float64Series: types.NewFloat64Series(),
		stddev:        StdDev(ClosePrices(source), window),
		window:        window,
	}

	s.Subscribe(s.stddev, func(v float64) {
		if s.count < s.window {
			s.count++
		}

		if s.count < s.window {
			return
		}

		s.PushAndEmit(v)
		s.Slice = generalTruncate(s.Slice)
	})

	return s
}

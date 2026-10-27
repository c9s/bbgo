package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// DEMAStream is the Double Exponential Moving Average
//
// Refer: Double Exponential Moving Average
// Refer URL: https://investopedia.com/terms/d/double-exponential-moving-average.asp
//
// The Double Exponential Moving Average (DEMA) is a technical analysis indicator that is used to smooth price data and reduce the lag
// associated with traditional moving averages. It is calculated by taking the exponentially weighted moving average of the input data,
// and then taking the exponentially weighted moving average of that result. This double-smoothing process helps to eliminate much of the noise
// in the original data and provides a more accurate representation of the underlying trend. The DEMA line is then plotted on the price chart,
// which can be used to make predictions about future price movements. The DEMA is typically more responsive to changes in the underlying data
// than a simple moving average, but may be less reliable in trending markets.
//
//	DEMA = 2 * EMA(x) - EMA(EMA(x))
type DEMAStream struct {
	*types.Float64Series

	window int

	// A1 is EMA(x), A2 is EMA(EMA(x))
	A1, A2 *EWMAStream
}

func DEMA(source types.Float64Source, window int) *DEMAStream {
	s := &DEMAStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		// the inner EWMA streams are driven by Calculate, so that they are always
		// updated before the DEMA value is computed (including history replay)
		A1: EWMA2(nil, window),
		A2: EWMA2(nil, window),
	}
	s.Bind(source, s)
	return s
}

func (s *DEMAStream) Calculate(v float64) float64 {
	s.A1.PushAndEmit(s.A1.Calculate(v))
	s.A1.Truncate()
	a1 := s.A1.Last(0)

	s.A2.PushAndEmit(s.A2.Calculate(a1))
	s.A2.Truncate()

	return 2*a1 - s.A2.Last(0)
}

func (s *DEMAStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}

package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// TEMAStream is the Triple Exponential Moving Average
//
// Refer: Triple Exponential Moving Average (TEMA)
// URL: https://investopedia.com/terms/t/triple-exponential-moving-average.asp
//
// The Triple Exponential Moving Average (TEMA) is a technical analysis indicator that is used to smooth price data and reduce the lag
// associated with traditional moving averages. It is calculated by taking the exponentially weighted moving average of the input data,
// and then taking the exponentially weighted moving average of that result, and then taking the exponentially weighted moving average of
// that result. This triple-smoothing process helps to eliminate much of the noise in the original data and provides a more accurate
// representation of the underlying trend. The TEMA line is then plotted on the price chart, which can be used to make predictions about
// future price movements. The TEMA is typically more responsive to changes in the underlying data than a simple moving average, but may be
// less reliable in trending markets.
//
//	TEMA = 3 * A1 - 3 * A2 + A3, where A1 = EMA(x), A2 = EMA(A1), A3 = EMA(A2)
type TEMAStream struct {
	*types.Float64Series

	window int

	A1, A2, A3 *EWMAStream
}

func TEMA(source types.Float64Source, window int) *TEMAStream {
	s := &TEMAStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		// the inner EWMA streams are driven by Calculate
		A1: EWMA2(nil, window),
		A2: EWMA2(nil, window),
		A3: EWMA2(nil, window),
	}
	s.Bind(source, s)
	return s
}

func (s *TEMAStream) Calculate(v float64) float64 {
	s.A1.PushAndEmit(s.A1.Calculate(v))
	s.A1.Truncate()
	a1 := s.A1.Last(0)

	s.A2.PushAndEmit(s.A2.Calculate(a1))
	s.A2.Truncate()
	a2 := s.A2.Last(0)

	s.A3.PushAndEmit(s.A3.Calculate(a2))
	s.A3.Truncate()
	a3 := s.A3.Last(0)

	return 3*a1 - 3*a2 + a3
}

func (s *TEMAStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}

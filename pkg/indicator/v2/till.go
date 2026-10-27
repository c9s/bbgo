package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// DefaultTILLVolumeFactor is the default volume factor of the Tillson T3 moving average
const DefaultTILLVolumeFactor = 0.7

// TILLStream is the Tillson T3 Moving Average
//
// Refer: Tillson T3 Moving Average
// Refer URL: https://tradingpedia.com/forex-trading-indicator/t3-moving-average-indicator/
//
// The Tillson T3 Moving Average (T3) is a technical analysis indicator that is used to smooth price data and reduce the lag associated
// with traditional moving averages. It was developed by Tim Tillson and is based on the exponential moving average, with the weighting
// factors determined using a modified version of the cubic polynomial. The T3 is calculated by taking the weighted moving average of the
// input data using weighting factors that are based on the standard deviation of the data and the specified length of the moving average.
// This resulting average is then plotted on the price chart as a line, which can be used to make predictions about future price movements.
// The T3 is typically more responsive to changes in the underlying data than a simple moving average, but may be less reliable in trending
// markets.
//
//	e1 = EMA(x), e2 = EMA(e1), ..., e6 = EMA(e5)
//	T3 = c1 * e6 + c2 * e5 + c3 * e4 + c4 * e3
type TILLStream struct {
	*types.Float64Series

	window       int
	volumeFactor float64

	e1, e2, e3, e4, e5, e6 *EWMAStream

	c1, c2, c3, c4 float64
}

// TILL creates the Tillson T3 moving average stream.
// When volumeFactor is 0, DefaultTILLVolumeFactor (0.7) is used.
func TILL(source types.Float64Source, window int, volumeFactor float64) *TILLStream {
	if volumeFactor == 0 {
		volumeFactor = DefaultTILLVolumeFactor
	}

	square := volumeFactor * volumeFactor
	cube := volumeFactor * square

	s := &TILLStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		volumeFactor:  volumeFactor,
		// the inner EWMA streams are driven by Calculate
		e1: EWMA2(nil, window),
		e2: EWMA2(nil, window),
		e3: EWMA2(nil, window),
		e4: EWMA2(nil, window),
		e5: EWMA2(nil, window),
		e6: EWMA2(nil, window),
		c1: -cube,
		c2: 3.*square + 3.*cube,
		c3: -6.*square - 3*volumeFactor - 3*cube,
		c4: 1. + 3.*volumeFactor + cube + 3.*square,
	}
	s.Bind(source, s)
	return s
}

func (s *TILLStream) Calculate(v float64) float64 {
	x := v
	for _, e := range []*EWMAStream{s.e1, s.e2, s.e3, s.e4, s.e5, s.e6} {
		e.PushAndEmit(e.Calculate(x))
		e.Truncate()
		x = e.Last(0)
	}

	return s.c1*s.e6.Last(0) + s.c2*s.e5.Last(0) + s.c3*s.e4.Last(0) + s.c4*s.e3.Last(0)
}

func (s *TILLStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}

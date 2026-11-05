package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// ZLEMAStream is the Zero Lag Exponential Moving Average
//
// Refer: Zero Lag Exponential Moving Average
// Refer URL: https://en.wikipedia.org/wiki/Zero_lag_exponential_moving_average
//
// The Zero Lag Exponential Moving Average (ZLEMA) is a technical analysis indicator that is used to smooth price data and reduce the
// lag associated with traditional moving averages. It is calculated by taking the exponentially weighted moving average of the input
// data, and then applying a digital filter to the resulting average to eliminate any remaining lag. This filtered average is then
// plotted on the price chart as a line, which can be used to make predictions about future price movements. The ZLEMA is typically more
// responsive to changes in the underlying data than a simple moving average, but may be less reliable in trending markets.
//
//	lag = round((window - 1) / 2)
//	ZLEMA = EMA(2 * x - x[lag], window)
//
// No value is emitted until more than `lag` inputs have been received.
type ZLEMAStream struct {
	*types.Float64Series

	window int
	lag    int

	data floats.Slice
	ewma *EWMAStream
}

func ZLEMA(source types.Float64Source, window int) *ZLEMAStream {
	s := &ZLEMAStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		lag:           int((float64(window)-1.)/2. + 0.5),
		ewma:          EWMA2(nil, window),
	}

	if source != nil {
		s.Subscribe(source, func(v float64) {
			if y, ok := s.calculate(v); ok {
				s.PushAndEmit(y)
				s.Truncate()
			}
		})
	}
	return s
}

func (s *ZLEMAStream) calculate(v float64) (float64, bool) {
	s.data.Push(v)
	s.data = types.ShrinkSlice(s.data, MaxSliceSize, TruncateSize)

	if s.lag >= s.data.Length() {
		return 0, false
	}

	emaData := 2.*v - s.data[len(s.data)-1-s.lag]
	s.ewma.PushAndEmit(s.ewma.Calculate(emaData))
	s.ewma.Truncate()
	return s.ewma.Last(0), true
}

func (s *ZLEMAStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}

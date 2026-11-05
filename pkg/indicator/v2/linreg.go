package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// LinRegStream is the Linear Regression baseline slope indicator.
//
// The values of the stream are the slopes of the linear regression baseline fitted
// on the last `window` input values. Before the window is filled, 0 is pushed.
//
// ValueRatios holds the ratio of the slope to the current input value (price),
// also 0 before the window is filled.
type LinRegStream struct {
	*types.Float64Series

	// ValueRatios are the ratios of slope to the price
	ValueRatios *types.Float64Series

	window int
	values []float64
}

// LinReg creates a linear regression baseline slope stream.
// To calculate on kline close prices, use LinReg(ClosePrices(kLines), window).
func LinReg(source types.Float64Source, window int) *LinRegStream {
	s := &LinRegStream{
		Float64Series: types.NewFloat64Series(),
		ValueRatios:   types.NewFloat64Series(),
		window:        window,
		values:        make([]float64, 0, window+1),
	}

	if source != nil {
		s.Subscribe(source, func(v float64) {
			slope := s.Calculate(v)

			ratio := 0.0
			if len(s.values) >= s.window {
				ratio = slope / v
			}

			s.ValueRatios.PushAndEmit(ratio)
			s.PushAndEmit(slope)
			s.Truncate()
		})
	}

	return s
}

// Calculate pushes v into the regression window and returns the slope of the
// linear regression baseline, or 0 if the window is not filled yet.
func (s *LinRegStream) Calculate(v float64) float64 {
	s.values = append(s.values, v)
	if len(s.values) > s.window {
		s.values = append(s.values[:0], s.values[len(s.values)-s.window:]...)
	}

	if len(s.values) < s.window {
		return 0
	}

	var sumX, sumY, sumXSqr, sumXY float64 = 0, 0, 0, 0
	end := len(s.values) - 1 // the last value
	for i := end; i >= end-s.window+1; i-- {
		val := s.values[i]
		per := float64(end - i + 1)
		sumX += per
		sumY += val
		sumXSqr += per * per
		sumXY += val * per
	}

	length := float64(s.window)
	slope := (length*sumXY - sumX*sumY) / (length*sumXSqr - sumX*sumX)
	average := sumY / length
	endPrice := average - slope*sumX/length + slope
	startPrice := endPrice + slope*(length-1)
	return (endPrice - startPrice) / (length - 1)
}

// LastRatio returns the last ratio of slope to price
func (s *LinRegStream) LastRatio() float64 {
	return s.ValueRatios.Last(0)
}

// IndexRatio returns the ratio of slope to price at the given index
func (s *LinRegStream) IndexRatio(i int) float64 {
	return s.ValueRatios.Last(i)
}

func (s *LinRegStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
	s.ValueRatios.Slice = generalTruncate(s.ValueRatios.Slice)
}

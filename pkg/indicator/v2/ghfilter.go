package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// GHFilterStream implements Ehler's Optimal Tracking Filter, an alpha-beta filter, also called g-h filter
//
// Refer: https://jamesgoulding.com/Research_II/Ehlers/Ehlers%20(Optimal%20Tracking%20Filters).doc
//
// The filter tracks the maneuverability uncertainty (EMA of the measurement change) and the
// measurement uncertainty (EMA of half of the uncertainty input), and derives the smoothing
// factor alpha from their ratio.
type GHFilterStream struct {
	*types.Float64Series

	window          int
	a               float64 // maneuverability uncertainty
	b               float64 // measurement uncertainty
	lastMeasurement float64
}

func newGHFilterStream(window int) *GHFilterStream {
	return &GHFilterStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
	}
}

// GHFilter creates a g-h filter on the kline close price, using the kline range
// (high - low) as the measurement uncertainty.
func GHFilter(source KLineSubscription, window int) *GHFilterStream {
	s := newGHFilterStream(window)
	if source != nil {
		source.AddSubscriber(func(k types.KLine) {
			s.PushAndEmit(s.update(k.Close.Float64(), k.High.Float64()-k.Low.Float64()))
			s.Truncate()
		})
	}
	return s
}

// GHFilterFromPrices creates a g-h filter on a float64 source, using the absolute change
// between the current and the last measurement as the measurement uncertainty.
func GHFilterFromPrices(source types.Float64Source, window int) *GHFilterStream {
	s := newGHFilterStream(window)
	s.Bind(source, s)
	return s
}

// Calculate updates the filter with the measurement v, using |v - last measurement| as the uncertainty
func (s *GHFilterStream) Calculate(v float64) float64 {
	return s.update(v, math.Abs(v-s.lastMeasurement))
}

func (s *GHFilterStream) update(value, uncertainty float64) float64 {
	if s.Length() == 0 {
		s.a = 0
		s.b = uncertainty / 2
		s.lastMeasurement = value
		return value
	}

	multiplier := 2.0 / float64(1+s.window) // EMA multiplier
	s.a = multiplier*(value-s.lastMeasurement) + (1-multiplier)*s.a
	s.b = multiplier*uncertainty/2 + (1-multiplier)*s.b
	lambda := s.a / s.b
	lambda2 := lambda * lambda
	alpha := (-lambda2 + math.Sqrt(lambda2*lambda2+16*lambda2)) / 8
	filtered := alpha*value + (1-alpha)*s.Slice.Last(0)
	s.lastMeasurement = value
	return filtered
}

func (s *GHFilterStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}

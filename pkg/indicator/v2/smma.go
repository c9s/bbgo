package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type SMMAStream struct {
	*types.Float64Series
	window    int
	rawValues *types.Queue
	source    types.Float64Source
}

func SMMA2(source types.Float64Source, window int) *SMMAStream {
	s := &SMMAStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		rawValues:     types.NewQueue(window),
		source:        source,
	}
	s.Bind(source, s)
	return s
}

func (s *SMMAStream) Calculate(v float64) float64 {
	var out float64
	sourceLen := s.source.Length()

	if sourceLen < s.window {
		// Until we reach the end of the period, sum the prices.

		// First, calculate the sum, and it will be automatically saved too.
		s.rawValues.Sum(s.window)
		// Then save the input value to use it later on.
		s.rawValues.Update(v)
	} else if sourceLen == s.window {
		// We need the SMA for the first time.
		s.rawValues.Update(v)
		out = s.rawValues.Mean(s.window)
	} else {
		// For all the rest values, just use the formula.
		last := s.Slice.Last(0)
		out = (last*float64((s.window-1.0)) + v) / float64(s.window)
	}

	return out
}

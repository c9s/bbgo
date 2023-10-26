package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type SquaredAverageStream struct {
	*types.Float64Series
	max    *MaxValueStream
	window int
}

func SquaredAverage(source types.Float64Source, window int) *SquaredAverageStream {
	s := &SquaredAverageStream{
		Float64Series: types.NewFloat64Series(),
		max:           MaxValue(source, window),
		window:        window,
	}

	s.Bind(source, s)

	return s
}

func (s *SquaredAverageStream) Calculate(v float64) float64 {
	var (
		max                = s.max.Last(0)
		percentageDrawdown = (v - max) / max * 100
		squaredAverage     = percentageDrawdown * percentageDrawdown
	)

	return squaredAverage
}

func (s *SquaredAverageStream) Truncate() {
	s.Slice = s.Slice.Truncate(s.window)
}

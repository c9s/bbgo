package indicatorv2

import "github.com/c9s/bbgo/pkg/types"

type StdDevStream struct {
	*types.Float64Series

	rawValues *types.Queue

	window     int
	multiplier float64
}

func StdDev(source types.Float64Source, window int) *StdDevStream {
	s := &StdDevStream{
		Float64Series: types.NewFloat64Series(),
		rawValues:     types.NewQueue(window),
		window:        window,
	}
	s.Bind(source, s)
	return s
}

func (s *StdDevStream) Calculate(x float64) float64 {
	s.rawValues.Update(x)
	var std = s.rawValues.Stdev()
	return std
}

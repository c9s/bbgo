package indicatorv2

import "github.com/c9s/bbgo/pkg/types"

type EWMAStream struct {
	*types.Float64Series

	window     int
	multiplier float64
}

func EWMA2(source types.Float64Source, window int) *EWMAStream {
	s := &EWMAStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		multiplier:    2.0 / float64(1+window),
	}
	s.Bind(source, s)
	return s
}

func (s *EWMAStream) Calculate(v float64) float64 {
	last := s.Slice.Last(0)
	if last == 0.0 {
		return v
	}

	m := s.multiplier
	return (1.0-m)*last + m*v
}

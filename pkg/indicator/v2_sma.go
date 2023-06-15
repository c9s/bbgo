package indicator

import "github.com/c9s/bbgo/pkg/types"

type SMAStream struct {
	*Float64Series
	window    int
	rawValues *types.Queue
}

func SMA2(source Float64Source, window int) *SMAStream {
	s := &SMAStream{
		Float64Series: NewFloat64Series(),
		window:        window,
		rawValues:     types.NewQueue(window),
	}
	s.Bind(source, s)
	return s
}

func (s *SMAStream) Calculate(v float64) float64 {
	s.rawValues.Update(v)
	sma := s.rawValues.Mean(s.window)
	return sma
}

func (s *SMAStream) Truncate() {
	s.slice = s.slice.Truncate(MaxNumOfSMA)
}

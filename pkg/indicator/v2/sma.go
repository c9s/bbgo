package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

const MaxNumOfSMA = 5_000

type SMAStream struct {
	*types.Float64Series
	window    int
	rawValues *types.Queue
}

func SMA(source types.Float64Source, window int) *SMAStream {
	s := &SMAStream{
		Float64Series: types.NewFloat64Series(),
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
	s.Slice = s.Slice.Truncate(MaxNumOfSMA)
}

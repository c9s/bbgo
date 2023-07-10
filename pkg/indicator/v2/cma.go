package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

type CMAStream struct {
	*types.Float64Series
}

func CMA2(source types.Float64Source) *CMAStream {
	s := &CMAStream{
		Float64Series: types.NewFloat64Series(),
	}
	s.Bind(source, s)
	return s
}

func (s *CMAStream) Calculate(x float64) float64 {
	l := float64(s.Slice.Length())
	cma := (s.Slice.Last(0)*l + x) / (l + 1.)
	return cma
}

func (s *CMAStream) Truncate() {
	s.Slice.Truncate(indicator.MaxNumOfEWMA)
}

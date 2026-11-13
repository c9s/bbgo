package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type CMAStream struct {
	*types.Float64Series

	// count is the number of values accumulated, tracked separately so that
	// truncating the output slice does not affect the cumulative average.
	count float64
}

func CMA2(source types.Float64Source) *CMAStream {
	s := &CMAStream{
		Float64Series: types.NewFloat64Series(),
	}
	s.Bind(source, s)
	return s
}

func (s *CMAStream) Calculate(x float64) float64 {
	cma := (s.Slice.Last(0)*s.count + x) / (s.count + 1.)
	s.count++
	return cma
}

func (s *CMAStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}

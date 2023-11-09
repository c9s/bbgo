package indicatorv2

import "github.com/c9s/bbgo/pkg/types"

type TrimaStream struct {
	// embedded struct
	*types.Float64Series

	trima *SMAStream
}

// Trima function calculates the Triangular Moving Average (TRIMA).
//
// If period is even:
//
//	TRIMA = SMA(period / 2, SMA((period / 2) + 1, values))
//
// If period is odd:
//
//	TRIMA = SMA((period + 1) / 2, SMA((period + 1) / 2, values))
//
// Returns trima.
func Trima(source types.Float64Source, window int) *TrimaStream {
	var n1, n2 int

	if window%2 == 0 {
		n1 = window / 2
		n2 = n1 + 1
	} else {
		n1 = (window + 1) / 2
		n2 = n1
	}

	var s = &TrimaStream{
		Float64Series: types.NewFloat64Series(),
		trima:         SMA(SMA(source, n2), n1),
	}

	s.Bind(source, s)
	return s
}

func (s *TrimaStream) Calculate(_ float64) float64 {
	return s.trima.Last(0)
}

package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// SSFStream implements Ehler's Super Smoother Filter
//
// Refer: https://easylanguagemastery.com/indicators/predictive-indicators/
// Refer: https://github.com/twopirllc/pandas-ta/blob/main/pandas_ta/overlap/ssf.py
//
// John F. Ehlers's solution to reduce lag and remove aliasing noise with his
// research in aerospace analog filter design. This indicator comes with two
// versions determined by the keyword poles. By default, it uses two poles but
// there is an option for three poles. Since SSF is a (Recursive) Digital Filter,
// the number of poles determine how many prior recursive SSF bars to include in
// the design of the filter. So two poles uses two prior SSF bars and three poles
// uses three prior SSF bars for their filter calculations.
type SSFStream struct {
	*types.Float64Series

	window int
	poles  int

	c1, c2, c3, c4 float64
}

// SSF creates a super smoother filter stream.
// poles can be 2 or 3; any value other than 3 is treated as 2.
func SSF(source types.Float64Source, window int, poles int) *SSFStream {
	s := &SSFStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		poles:         poles,
	}

	if poles == 3 {
		x := math.Pi / float64(window)
		a0 := math.Exp(-x)
		b0 := 2. * a0 * math.Cos(math.Sqrt(3.)*x)
		c0 := a0 * a0

		s.c4 = c0 * c0
		s.c3 = -c0 * (1. + b0)
		s.c2 = c0 + b0
		s.c1 = 1. - s.c2 - s.c3 - s.c4
	} else {
		x := math.Pi * math.Sqrt(2.) / float64(window)
		a0 := math.Exp(-x)
		s.c3 = -a0 * a0
		s.c2 = 2. * a0 * math.Cos(x)
		s.c1 = 1. - s.c2 - s.c3
	}

	s.Bind(source, s)
	return s
}

func (s *SSFStream) Calculate(v float64) float64 {
	if s.poles == 3 {
		return s.c1*v +
			s.c2*s.Slice.Last(0) +
			s.c3*s.Slice.Last(1) +
			s.c4*s.Slice.Last(2)
	}

	return s.c1*v +
		s.c2*s.Slice.Last(0) +
		s.c3*s.Slice.Last(1)
}

func (s *SSFStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}

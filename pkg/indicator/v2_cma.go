package indicator

type CMAStream struct {
	Float64Series
}

func CMA2(source Float64Source) *CMAStream {
	s := &CMAStream{
		Float64Series: NewFloat64Series(),
	}
	s.Bind(source, s)
	return s
}

func (s *CMAStream) Calculate(x float64) float64 {
	l := float64(s.slice.Length())
	cma := (s.slice.Last(0)*l + x) / (l + 1.)
	return cma
}

func (s *CMAStream) Truncate() {
	s.slice.Truncate(MaxNumOfEWMA)
}

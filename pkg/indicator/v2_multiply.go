package indicator

type MultiplyStream struct {
	Float64Series
	multiplier float64
}

func Multiply(source Float64Source, multiplier float64) *MultiplyStream {
	s := &MultiplyStream{
		Float64Series: NewFloat64Series(),
		multiplier:    multiplier,
	}
	s.Bind(source, s)
	return s
}

func (s *MultiplyStream) Calculate(v float64) float64 {
	return v * s.multiplier
}

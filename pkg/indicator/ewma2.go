package indicator

type EWMAStream struct {
	Float64Series

	window     int
	multiplier float64
}

func EWMA2(source Float64Source, window int) *EWMAStream {
	s := &EWMAStream{
		Float64Series: NewFloat64Series(),
		window:        window,
		multiplier:    2.0 / float64(1+window),
	}

	if sub, ok := source.(Float64Subscription); ok {
		sub.AddSubscriber(s.calculateAndPush)
	} else {
		source.OnUpdate(s.calculateAndPush)
	}

	return s
}

func (s *EWMAStream) calculateAndPush(v float64) {
	v2 := s.calculate(v)
	s.slice.Push(v2)
	s.EmitUpdate(v2)
}

func (s *EWMAStream) calculate(v float64) float64 {
	last := s.slice.Last()
	m := s.multiplier
	return (1.0-m)*last + m*v
}

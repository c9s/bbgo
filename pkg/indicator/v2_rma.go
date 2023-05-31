package indicator

type RMAStream struct {
	// embedded structs
	Float64Series

	// config fields
	Adjust bool

	window        int
	counter       int
	sum, previous float64
}

func RMA2(source Float64Source, window int, adjust bool) *RMAStream {
	s := &RMAStream{
		Float64Series: NewFloat64Series(),
		window:        window,
		Adjust:        adjust,
	}

	if sub, ok := source.(Float64Subscription); ok {
		sub.AddSubscriber(s.calculateAndPush)
	} else {
		source.OnUpdate(s.calculateAndPush)
	}

	return s
}

func (s *RMAStream) calculateAndPush(v float64) {
	v2 := s.calculate(v)
	s.slice.Push(v2)
	s.EmitUpdate(v2)
	s.truncate()
}

func (s *RMAStream) calculate(x float64) float64 {
	lambda := 1 / float64(s.window)
	tmp := 0.0
	if s.counter == 0 {
		s.sum = 1
		tmp = x
	} else {
		if s.Adjust {
			s.sum = s.sum*(1-lambda) + 1
			tmp = s.previous + (x-s.previous)/s.sum
		} else {
			tmp = s.previous*(1-lambda) + x*lambda
		}
	}
	s.counter++

	if s.counter < s.window {
		// we can use x, but we need to use 0. to make the same behavior as the result from python pandas_ta
		s.slice.Push(0)
	}

	s.slice.Push(tmp)
	s.previous = tmp

	return tmp
}

func (s *RMAStream) truncate() {
	if len(s.slice) > MaxNumOfRMA {
		s.slice = s.slice[MaxNumOfRMATruncateSize-1:]
	}
}

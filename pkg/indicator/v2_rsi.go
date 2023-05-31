package indicator

type RSIStream struct {
	// embedded structs
	Float64Series

	// config fields
	window int

	// private states
}

func RSI2(source Float64Source, window int) *RSIStream {
	s := &RSIStream{
		Float64Series: NewFloat64Series(),
		window:        window,
	}

	if sub, ok := source.(Float64Subscription); ok {
		sub.AddSubscriber(s.calculateAndPush)
	} else {
		source.OnUpdate(s.calculateAndPush)
	}

	return s
}

func (s *RSIStream) calculateAndPush(x float64) {

}

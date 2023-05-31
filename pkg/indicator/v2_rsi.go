package indicator

type RSIStream struct {
	// embedded structs
	Float64Series

	// config fields
	window int

	// private states
	source Float64Source
}

func RSI2(source Float64Source, window int) *RSIStream {
	s := &RSIStream{
		source:        source,
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

func (s *RSIStream) calculate(_ float64) float64 {
	var gainSum, lossSum float64
	var sourceLen = s.source.Length()
	var limit = min(s.window, sourceLen)
	for i := 0; i < limit; i++ {
		value := s.source.Index(i)
		prev := s.source.Index(i + 1)
		change := value - prev
		if change >= 0 {
			gainSum += change
		} else {
			lossSum += -change
		}
	}

	avgGain := gainSum / float64(limit)
	avgLoss := lossSum / float64(limit)
	rs := avgGain / avgLoss
	rsi := 100.0 - (100.0 / (1.0 + rs))
	return rsi
}

func (s *RSIStream) calculateAndPush(x float64) {
	rsi := s.calculate(x)
	s.slice.Push(rsi)
	s.EmitUpdate(rsi)
}

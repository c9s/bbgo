package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type RSIStream struct {
	// embedded structs
	*types.Float64Series

	// config fields
	window int

	// private states
	source types.Float64Source
}

func RSI2(source types.Float64Source, window int) *RSIStream {
	s := &RSIStream{
		source:        source,
		Float64Series: types.NewFloat64Series(),
		window:        window,
	}
	s.Bind(source, s)
	return s
}

func (s *RSIStream) Calculate(_ float64) float64 {
	var gainSum, lossSum float64
	var sourceLen = s.source.Length()
	var limit = min(s.window, sourceLen)
	for i := 0; i < limit; i++ {
		value := s.source.Last(i)
		prev := s.source.Last(i + 1)
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

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

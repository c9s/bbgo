package indicator

import (
	"github.com/c9s/bbgo/pkg/types"
)

type ATRStream struct {
	Float64Updater

	types.SeriesBase

	window     int
	multiplier float64
}

func ATR2(source KLineSubscription, window int) *ATRStream {
	s := &ATRStream{
		window:     window,
		multiplier: 2.0 / float64(1+window),
	}

	s.SeriesBase.Series = s.slice

	source.AddSubscriber(func(k types.KLine) {
		// v := s.mapper(k)
		// s.slice.Push(v)
		// s.EmitUpdate(v)
	})

	return s
}

func (s *ATRStream) calculateAndPush(k types.KLine) {
	// v2 := s.calculate(v)
	// s.slice.Push(v2)
	// s.EmitUpdate(v2)
}

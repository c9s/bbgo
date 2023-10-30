package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type EngulfingStream struct {
	*types.Float64Series

	window int
}

// Bullish engulfing patterns are more likely to signal reversals when they are preceded by four or more black candlesticks.
func Engulfing(source KLineSubscription) *EngulfingStream {
	s := &EngulfingStream{
		Float64Series: types.NewFloat64Series(),
		window:        2,
	}

	source.AddSubscriber(func(kLine types.KLine) {
		var (
			i      = source.Length()
			output = Neutral
		)
		if i < s.window {
			s.PushAndEmit(output)
			return
		}
		var (
			two = source.Last(1)
			one = source.Last(0)
		)

		// BEAR
		if two.Open < two.Close {
			if one.Open > two.Close {
				if one.Close < two.Open {
					output = Bear
				}
			}
		}

		// BULL
		if two.Open > two.Close {
			if one.Open < two.Close {
				if one.Close > two.Open {
					output = Bull
				}
			}
		}

		s.PushAndEmit(output)

	})

	return s
}

func (s *EngulfingStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

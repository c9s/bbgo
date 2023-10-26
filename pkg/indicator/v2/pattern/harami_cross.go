package pattern

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type HaramiCrossStream struct {
	*types.Float64Series

	window int
}

func HaramiCross(source v2.KLineSubscription, direction Direction, maxDiff float64) *HaramiCrossStream {
	s := &HaramiCrossStream{
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
			two        = source.Last(1)
			one        = source.Last(0)
			isLastDoji = fixedpoint.ApproxEqual(one.Open, one.Close, maxDiff)
		)
		if direction == Bullish {
			var (
				isBullishHaramiCrossPattern = two.Open > one.Open &&
					two.Close < one.Open &&
					two.Close < one.Close &&
					two.Open > one.Low &&
					two.High > one.High
			)
			if isBullishHaramiCrossPattern && isLastDoji {
				output = Bull
			}
		} else {
			var isBearishHaramiCrossPattern = two.Open < one.Open &&
				two.Close > one.Open &&
				two.Close > one.Close &&
				two.Open < one.Low &&
				two.High > one.High
			if isBearishHaramiCrossPattern && isLastDoji {
				output = Bear
			}
		}

		s.PushAndEmit(output)

	})

	return s
}

func (s *HaramiCrossStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

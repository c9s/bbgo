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

func HaramiCross(source v2.KLineSubscription, maxDiff float64) *HaramiCrossStream {
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
			two                         = source.Last(1)
			one                         = source.Last(0)
			isSecondDayDoji             = fixedpoint.ApproxEqual(one.Open, one.Close, maxDiff)
			isBullishHaramiCrossPattern = two.Open > one.Open &&
				two.Close > one.Open &&
				two.Close > one.Close &&
				two.Open < one.Low &&
				two.High < one.High
		)
		if isBullishHaramiCrossPattern && isSecondDayDoji {
			output = Bull
		} else {
			var isBearishHaramiCrossPattern = two.Open < one.Open &&
				two.Close < one.Open &&
				two.Close < one.Close &&
				two.Open > one.Low &&
				two.High > one.High
			if isBearishHaramiCrossPattern && isSecondDayDoji {
				output = Bear
			}
		}

		s.PushAndEmit(output)

	})

	return s
}

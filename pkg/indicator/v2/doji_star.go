package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"

	"github.com/c9s/bbgo/pkg/types"
)

type DojiStarStream struct {
	*types.Float64Series

	window int
}

// maxDiff is the maximum deviation between a and b to consider them approximately equal
func DojiStar(source KLineSubscription, direction Direction, maxDiff float64) *DojiStarStream {
	s := &DojiStarStream{
		Float64Series: types.NewFloat64Series(),
		window:        3,
	}
	var doji = Doji(source, maxDiff)

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
			three         = source.Last(2)
			two           = source.Last(1)
			one           = source.Last(0)
			firstMidpoint = three.Open.Add(three.Close).Div(fixedpoint.Two)
			dojiExists    = doji.Last(1) == Bull
		)
		if direction == Bullish {
			var (
				isFirstBearish = three.Close < three.Open
				isThirdBullish = one.Close > one.Open
				gapExists      = two.High < three.Low &&
					two.Low < three.Low &&
					one.Open > two.High &&
					two.Close < one.Open
				doesCloseAboveFirstMidpoint = one.Close > firstMidpoint
			)

			if isFirstBearish && dojiExists && isThirdBullish && gapExists && doesCloseAboveFirstMidpoint {
				output = Bull
			}
		} else {
			var (
				isFirstBullish = three.Close > three.Open
				isThirdBearish = one.Open > one.Close
				gapExists      = two.High > three.High &&
					two.Low > three.High &&
					one.Open < two.Low &&
					two.Close > one.Open
				doesCloseBelowFirstMidpoint = one.Close < firstMidpoint
			)

			if isFirstBullish && dojiExists && gapExists && isThirdBearish && doesCloseBelowFirstMidpoint {
				output = Bear
			}
		}

		s.PushAndEmit(output)

	})

	return s
}

func (s *DojiStarStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

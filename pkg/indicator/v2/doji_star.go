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
			firstMidpoint = three.Open.Add(three.Close).Div(fixedpoint.Two).Float64()
			dojiExists    = doji.Last(1) == Bull
		)
		if direction == Bullish {
			var (
				isFirstBearish = three.Close.Float64() < three.Open.Float64()
				isThirdBullish = one.Close.Float64() > one.Open.Float64()
				gapExists      = two.High.Float64() < three.Low.Float64() &&
					two.Low.Float64() < three.Low.Float64() &&
					one.Open.Float64() > two.High.Float64() &&
					two.Close.Float64() < one.Open.Float64()
				doesCloseAboveFirstMidpoint = one.Close.Float64() > firstMidpoint
			)

			if isFirstBearish && dojiExists && isThirdBullish && gapExists && doesCloseAboveFirstMidpoint {
				output = Bull
			}
		} else {
			var (
				isFirstBullish = three.Close.Float64() > three.Open.Float64()
				isThirdBearish = one.Open.Float64() > one.Close.Float64()
				gapExists      = two.High.Float64() > three.High.Float64() &&
					two.Low.Float64() > three.High.Float64() &&
					one.Open.Float64() < two.Low.Float64() &&
					two.Close.Float64() > one.Open.Float64()
				doesCloseBelowFirstMidpoint = one.Close.Float64() < firstMidpoint
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

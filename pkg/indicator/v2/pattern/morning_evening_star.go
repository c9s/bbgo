package pattern

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type MorningOrEveningStarStream struct {
	*types.Float64Series

	window int
}

func MorningOrEveningStar(source v2.KLineSubscription, direction Direction) *MorningOrEveningStarStream {
	s := &MorningOrEveningStarStream{
		Float64Series: types.NewFloat64Series(),
		window:        3,
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
			three         = source.Last(2)
			two           = source.Last(1)
			one           = source.Last(0)
			firstMidpoint = three.Open.Add(three.Close).Div(fixedpoint.Two)
		)
		if direction == Bullish {
			var (
				isFirstBearish = three.Close < three.Open
				hasSmallBody   = three.Low > two.Low &&
					three.Low > two.High
				isThirdBullish = one.Open < one.Close
				gapExists      = two.High < three.Low &&
					two.Low < three.Low &&
					one.Open > two.High &&
					two.Close < one.Open
				doesCloseAboveFirstMidpoint = one.Close > firstMidpoint
			)
			if isFirstBearish && hasSmallBody && gapExists && isThirdBullish && doesCloseAboveFirstMidpoint {
				output = Bull // morning star
			}
		} else {
			var (
				isFirstBullish = three.Close > three.Open
				hasSmallBody   = three.High < two.Low &&
					three.High < two.High
				isThirdBearish = one.Open > one.Close
				gapExists      = two.High > three.High &&
					two.Low > three.High &&
					one.Open < two.Low &&
					two.Close > one.Open
				doesCloseBelowFirstMidpoint = one.Close < firstMidpoint
			)
			if isFirstBullish && hasSmallBody && gapExists && isThirdBearish && doesCloseBelowFirstMidpoint {
				output = Bear // evening star
			}
		}

		s.PushAndEmit(output)

	})

	return s
}

func (s *MorningOrEveningStarStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"

	"github.com/c9s/bbgo/pkg/types"
)

type MorningOrEveningStarStream struct {
	*types.Float64Series

	window int
}

// An evening star is a candlestick pattern that's used by technical analysts to predict future price reversals
// to the downside.
// The evening star pattern is rare but it's considered by traders to be a reliable technical indicator.
// The evening star is the opposite of the morning star.
// The candlestick pattern is bearish whereas the morning star pattern is bullish.
func MorningOrEveningStar(source KLineSubscription, direction Direction) *MorningOrEveningStarStream {
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

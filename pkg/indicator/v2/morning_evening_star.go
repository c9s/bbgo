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
			firstMidpoint = three.Open.Add(three.Close).Div(fixedpoint.Two).Float64()
		)
		if direction == Bullish {
			var (
				isFirstBearish = three.Close.Float64() < three.Open.Float64()
				hasSmallBody   = three.Low.Float64() > two.Low.Float64() &&
					three.Low.Float64() > two.High.Float64()
				isThirdBullish = one.Open.Float64() < one.Close.Float64()
				gapExists      = two.High.Float64() < three.Low.Float64() &&
					two.Low.Float64() < three.Low.Float64() &&
					one.Open.Float64() > two.High.Float64() &&
					two.Close.Float64() < one.Open.Float64()
				doesCloseAboveFirstMidpoint = one.Close.Float64() > firstMidpoint
			)
			if isFirstBearish && hasSmallBody && gapExists && isThirdBullish && doesCloseAboveFirstMidpoint {
				output = Bull // morning star
			}
		} else {
			var (
				isFirstBullish = three.Close.Float64() > three.Open.Float64()
				hasSmallBody   = three.High.Float64() < two.Low.Float64() &&
					three.High.Float64() < two.High.Float64()
				isThirdBearish = one.Open.Float64() > one.Close.Float64()
				gapExists      = two.High.Float64() > three.High.Float64() &&
					two.Low.Float64() > three.High.Float64() &&
					one.Open.Float64() < two.Low.Float64() &&
					two.Close.Float64() > one.Open.Float64()
				doesCloseBelowFirstMidpoint = one.Close.Float64() < firstMidpoint
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

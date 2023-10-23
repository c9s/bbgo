package pattern

import (
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type DojiStarStream struct {
	*types.Float64Series

	window int
}

// maxDiff is the maximum deviation between a and b to consider them approximately equal
func DojiStar(source v2.KLineSubscription, maxDiff float64) *DojiStarStream {
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
			three          = source.Last(2)
			two            = source.Last(1)
			one            = source.Last(0)
			firstMidpoint  = (three.Open + three.Close) / 2
			isFirstBearish = three.Close < three.Open
			isThirdBullish = one.Close > one.Open
			gapExists      = two.High < three.Low &&
				two.Low < three.Low &&
				one.Open > two.High &&
				two.Close < one.Open
			doesCloseAboveFirstMidpoint = one.Close > firstMidpoint
		)
		var dojiExists = doji.Last(0) == Bull
		if isFirstBearish && dojiExists && isThirdBullish && gapExists && doesCloseAboveFirstMidpoint {
			output = Bull
		} else {
			var (
				isFirstBullish = two.Close > two.Open
				isThirdBearish = one.Open > one.Close
			)
			gapExists = two.High > three.High &&
				two.Low > three.High &&
				one.Open < two.Low &&
				two.Close > one.Open
			var doesCloseBelowFirstMidpoint = one.Close < firstMidpoint
			if isFirstBullish && dojiExists && gapExists && isThirdBearish && doesCloseBelowFirstMidpoint {
				output = Bear
			}

		}

		s.PushAndEmit(output)

	})

	return s
}

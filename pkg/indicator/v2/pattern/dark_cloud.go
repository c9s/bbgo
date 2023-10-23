package pattern

import (
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type DarkCloudStream struct {
	*types.Float64Series

	window int
}

func DarkCloud(source v2.KLineSubscription) *DarkCloudStream {
	s := &DarkCloudStream{
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
			two             = source.Last(1)
			one             = source.Last(0)
			twoMidpoint     = (two.Close + two.Open) / 2
			isFirstBullish  = two.Close > two.Open
			isSecondBearish = one.Close < one.Open
			isDarkCloud     = one.Open > two.High &&
				one.Close < twoMidpoint && one.Close > two.Open
		)

		if isFirstBullish && isSecondBearish && isDarkCloud {
			output = Bear
		}

		s.PushAndEmit(output)

	})

	return s
}

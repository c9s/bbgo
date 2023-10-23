package pattern

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type AbondonedBabyStream struct {
	*types.Float64Series

	window int
}

func AbondonedBaby(source v2.KLineSubscription) *AbondonedBabyStream {
	s := &AbondonedBabyStream{
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
			one   = source.Last(2)
			two   = source.Last(1)
			three = source.Last(0)
		)

		if one.Open < one.Close {
			if one.High < two.Low {
				if fixedpoint.Abs((two.Close-two.Open)/(two.Open)) < threshold {
					if three.Open < two.Low && three.Close < three.Open {
						output = -1.0
					}
				}
			}
		}

		if one.Open > one.Close {
			if one.Low > two.High {
				if fixedpoint.Abs((two.Close-two.Open)/two.Open) <= threshold {
					if three.Open > two.High && three.Close > three.Open {
						output = 1.0
					}
				}
			}
		}

		s.PushAndEmit(output)

	})

	return s
}

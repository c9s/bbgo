package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type AbondonedBabyStream struct {
	*types.Float64Series

	window int
}

func AbondonedBaby(source KLineSubscription) *AbondonedBabyStream {
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
			abs   = fixedpoint.Abs((two.Close.Sub(two.Open).Div(two.Open))).Float64()
		)

		if one.Open.Float64() < one.Close.Float64() {
			if one.High.Float64() < two.Low.Float64() {
				if abs < threshold {
					if three.Open.Float64() < two.Low.Float64() &&
						three.Close.Float64() < three.Open.Float64() {
						output = -1.0
					}
				}
			}
		}

		if one.Open.Float64() > one.Close.Float64() {
			if one.Low.Float64() > two.High.Float64() {
				if abs <= threshold {
					if three.Open.Float64() > two.High.Float64() &&
						three.Close.Float64() > three.Open.Float64() {
						output = 1.0
					}
				}
			}
		}

		s.Float64Series.PushAndEmit(output)

	})

	return s
}

func (s *AbondonedBabyStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type BeltholdStream struct {
	*types.Float64Series

	window int
}

func Belthold(source KLineSubscription) *BeltholdStream {
	s := &BeltholdStream{
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
			two = source.Last(1)
			one = source.Last(0)
		)

		if two.Close.Float64() > two.Open.Float64() {
			if two.High.Float64() < one.Open.Float64() {
				if one.Open.Float64() == one.High.Float64() {
					if one.Close.Float64() < one.Open.Float64() {
						output = Bear
					}
				}
			}
		}

		if two.Close.Float64() < two.Open.Float64() {
			if two.Low.Float64() > one.Open.Float64() {
				if one.Open.Float64() == one.Low.Float64() {
					if one.Close.Float64() > one.Open.Float64() {
						output = Bull
					}
				}
			}
		}

		s.PushAndEmit(output)

	})

	return s
}

func (s *BeltholdStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

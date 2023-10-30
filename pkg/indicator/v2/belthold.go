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

		if two.Close > two.Open {
			if two.High < one.Open {
				if one.Open == one.High {
					if one.Close < one.Open {
						output = Bear
					}
				}
			}
		}

		if two.Close < two.Open {
			if two.Low > one.Open {
				if one.Open == one.Low {
					if one.Close > one.Open {
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

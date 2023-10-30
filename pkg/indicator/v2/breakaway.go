package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type BreakAwayStream struct {
	*types.Float64Series

	window int
}

func BreakAway(source KLineSubscription) *BreakAwayStream {
	s := &BreakAwayStream{
		Float64Series: types.NewFloat64Series(),
		window:        5,
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
			five  = source.Last(4)
			four  = source.Last(3)
			three = source.Last(2)
			two   = source.Last(1)
			one   = source.Last(0)
		)

		if five.Open < five.Close {
			if four.Open < four.Close && five.Close < four.Open {
				if four.Close < three.Close && three.Close < two.Close {
					if one.Open > one.Close && one.Close > five.Close {
						output = Bear
					}
				}
			}
		}

		if five.Open > five.Close {
			if four.Open > four.Close && five.Close > four.Open {
				if four.Close > three.Close && three.Close > two.Close {
					if one.Open < one.Close && one.Close < five.Close {
						output = Bull
					}
				}
			}
		}

		s.PushAndEmit(output)

	})

	return s
}

func (s *BreakAwayStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type ThreeLineStrikeStream struct {
	*types.Float64Series

	window int
}

func ThreeLineStrike(source KLineSubscription) *ThreeLineStrikeStream {
	s := &ThreeLineStrikeStream{
		Float64Series: types.NewFloat64Series(),
		window:        4,
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
			four  = source.Last(3)
			three = source.Last(2)
			two   = source.Last(1)
			one   = source.Last(0)
		)
		// BEAR
		if three.Close < four.Close {
			if two.Close < three.Close {
				if four.Close < three.Open && three.Open < four.Open {
					if three.Close < two.Open && two.Open < three.Open {
						if one.Open < two.Close {
							if one.Close > four.Open {
								output = Bear
							}
						}
					}
				}

			}
		}

		// BULL
		if three.Close > four.Close {
			if two.Close > three.Close {
				if four.Close > three.Open && three.Open > four.Open {
					if three.Close > two.Open && two.Open > three.Open {
						if one.Open > two.Close {
							if one.Close < four.Open {
								output = Bull
							}
						}
					}
				}

			}
		}

		s.PushAndEmit(output)

	})

	return s
}

func (s *ThreeLineStrikeStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

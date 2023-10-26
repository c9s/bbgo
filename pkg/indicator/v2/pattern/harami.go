package pattern

import (
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type HaramiStream struct {
	*types.Float64Series

	window int
}

func Harami(source v2.KLineSubscription) *HaramiStream {
	s := &HaramiStream{
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

		// BEAR
		if two.Open < two.Close {
			if one.Open > one.Close {
				if one.Open < two.Close && one.Close > two.Open {
					output = Bear
				}
			}
		}

		// BULL
		if two.Open > two.Close {
			if one.Open < one.Close {
				if one.Open > two.Close && one.Close < two.Open {
					output = Bull
				}
			}
		}
		s.PushAndEmit(output)

	})

	return s
}

func (s *HaramiStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

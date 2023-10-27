package pattern

import (
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type HaramiStream struct {
	*types.Float64Series

	window int
}

// The bullish harami indicator is charted as a long candlestick followed by a smaller body,
// referred to as a doji, that is completely contained within the vertical range of the previous body.
// To some, a line drawn around this pattern resembles a pregnant woman. The word harami comes from an
// old Japanese word meaning pregnant.
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

package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"

	"github.com/c9s/bbgo/pkg/types"
)

type KickingStream struct {
	*types.Float64Series

	window int
}

func Kicking(source KLineSubscription, maxDiff float64) *KickingStream {
	s := &KickingStream{
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
				if two.Open > one.Open {
					if fixedpoint.ApproxEqual(two.Open, two.Low, maxDiff) &&
						fixedpoint.ApproxEqual(two.Close, two.High, maxDiff) {
						if fixedpoint.ApproxEqual(one.Open, one.High, maxDiff) &&
							fixedpoint.ApproxEqual(one.Close, one.Low, maxDiff) {
							output = Bear
						}
					}
				}
			}
		}

		// BULL
		if two.Open > two.Close {
			if one.Open < one.Close {
				if two.Open < one.Open {
					if fixedpoint.ApproxEqual(two.Open, two.High, maxDiff) &&
						fixedpoint.ApproxEqual(two.Close, two.Low, maxDiff) {
						if fixedpoint.ApproxEqual(one.Open, one.Low, maxDiff) &&
							fixedpoint.ApproxEqual(one.Close, one.High, maxDiff) {
							output = Bull
						}
					}
				}
			}
		}

		s.PushAndEmit(output)

	})

	return s
}

func (s *KickingStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

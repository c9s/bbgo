package pattern

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type SeparatingLinesStream struct {
	*types.Float64Series

	window int
}

func SeparatingLines(source v2.KLineSubscription, maxDiff float64) *SeparatingLinesStream {
	s := &SeparatingLinesStream{
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
			three = source.Last(2)
			two   = source.Last(1)
			one   = source.Last(0)
		)
		// BEAR
		if three.Open > three.Close {
			if two.Open < two.Close {
				if one.Open > one.Close {
					if fixedpoint.Abs((two.Open-one.Open)/one.Open) < threshold {
						if fixedpoint.ApproxEqual(one.Open, one.High, maxDiff) {
							output = Bear
						}
					}
				}
			}
		}

		// BULL
		if three.Open < three.Close {
			if two.Open > two.Close {
				if one.Open < one.Close {
					if fixedpoint.Abs((two.Open-one.Open)/one.Open) < threshold {
						if fixedpoint.ApproxEqual(one.Open, one.Low, maxDiff) {
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

func (s *SeparatingLinesStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

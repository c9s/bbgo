package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type SeparatingLinesStream struct {
	*types.Float64Series

	window int
}

func SeparatingLines(source KLineSubscription, maxDiff float64) *SeparatingLinesStream {
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
		if three.Open.Float64() > three.Close.Float64() {
			if two.Open.Float64() < two.Close.Float64() {
				if one.Open.Float64() > one.Close.Float64() {
					if fixedpoint.Abs(two.Open.Sub(one.Open).Div(one.Open)).Float64() < threshold {
						if fixedpoint.ApproxEqual(one.Open, one.High, maxDiff) {
							output = Bear
						}
					}
				}
			}
		}

		// BULL
		if three.Open.Float64() < three.Close.Float64() {
			if two.Open.Float64() > two.Close.Float64() {
				if one.Open.Float64() < one.Close.Float64() {
					if fixedpoint.Abs(two.Open.Sub(one.Open).Div(one.Open)).Float64() < threshold {
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

package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type SideBySideWhiteLinesStream struct {
	*types.Float64Series

	window int
}

func SideBySideWhiteLines(source KLineSubscription, maxDiff float64) *SideBySideWhiteLinesStream {
	s := &SideBySideWhiteLinesStream{
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
		if four.Open.Float64() > four.Close.Float64() {
			if three.Open.Float64() > three.Close.Float64() {
				if three.Low.Float64() > two.High.Float64() {
					if two.Open.Float64() < two.Close.Float64() {
						if one.Open.Float64() < one.Close.Float64() {
							if fixedpoint.Abs(two.Open.Sub(one.Open).Div(one.Open)).Float64() < threshold {
								if fixedpoint.Abs(two.Close.Sub(one.Close).Div(one.Close)).Float64() < threshold {
									if fixedpoint.ApproxEqual(two.Open, one.Open, maxDiff) {
										output = Bear
									}
								}
							}
						}
					}
				}
			}
		}

		// BULL
		if four.Open.Float64() < four.Close.Float64() {
			if three.Open.Float64() < three.Close.Float64() {
				if three.Low.Float64() < two.High.Float64() {
					if two.Open.Float64() < two.Close.Float64() {
						if one.Open.Float64() < one.Close.Float64() {
							if fixedpoint.Abs(two.Open.Sub(one.Open).Div(one.Open)).Float64() < threshold {
								if fixedpoint.Abs(two.Close.Sub(one.Close).Div(one.Close)).Float64() < threshold {
									if fixedpoint.ApproxEqual(two.Open, one.Open, maxDiff) {
										output = Bull
									}
								}
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

func (s *SideBySideWhiteLinesStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

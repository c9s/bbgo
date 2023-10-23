package pattern

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type SideBySideWhiteLinesStream struct {
	*types.Float64Series

	window int
}

func SideBySideWhiteLines(source v2.KLineSubscription, maxDiff float64) *SideBySideWhiteLinesStream {
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
		if four.Open > four.Close {
			if three.Open > three.Close {
				if three.Low > two.High {
					if two.Open < two.Close {
						if one.Open < one.Close {
							if fixedpoint.Abs((two.Open-one.Open)/one.Open) < threshold {
								if fixedpoint.Abs((two.Close-one.Close)/one.Close) < threshold {
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
		if four.Open < four.Close {
			if three.Open < three.Close {
				if three.Low < two.High {
					if two.Open < two.Close {
						if one.Open < one.Close {
							if fixedpoint.Abs((two.Open-one.Open)/one.Open) < threshold {
								if fixedpoint.Abs((two.Close-one.Close)/one.Close) < threshold {
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

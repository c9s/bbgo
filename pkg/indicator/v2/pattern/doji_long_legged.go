package pattern

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type DojiLongLeggedStream struct {
	*types.Float64Series

	window int
}

func DojiLongLegged(source v2.KLineSubscription) *DojiLongLeggedStream {
	s := &DojiLongLeggedStream{
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
		if four.Close > four.Open {
			if three.Close > three.Open {
				if two.Close > two.Open {
					if fixedpoint.Abs((one.Close-one.Open)/one.Open) < threshold {
						if fixedpoint.Abs((one.High-one.Open)/one.Open) > limit {
							if fixedpoint.Abs((one.Close-one.Low)/one.Low) > limit {
								output = Bear
							}
						}
					}
				}
			}
		}

		// BULL
		if four.Close < four.Open {
			if three.Close < three.Open {
				if two.Close < two.Open {
					if fixedpoint.Abs((one.Open-one.Close)/one.Close) < threshold {
						if fixedpoint.Abs((one.Low-one.Close)/one.Close) > limit {
							if fixedpoint.Abs((one.Open-one.High)/one.High) > limit {
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

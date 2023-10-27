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

// The long-legged doji is a type of candlestick pattern that signals to traders a point of indecision
// about the future direction of price. This doji has long upper and lower shadows and roughly
// the same opening and closing prices. In addition to signaling indecision, the long-legged doji can also
// indicate the beginning of a consolidation period where price action may soon break out to form a new trend.
// These doji can be a sign that sentiment is changing and that a trend reversal is on the horizon.
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
					if fixedpoint.Abs(one.Close.Sub(one.Open).Div(one.Open)) < threshold {
						if fixedpoint.Abs(one.High.Sub(one.Open).Div(one.Open)) > limit {
							if fixedpoint.Abs(one.Close.Sub(one.Low).Div(one.Low)) > limit {
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
					if fixedpoint.Abs(one.Open.Sub(one.Close).Div(one.Close)) < threshold {
						if fixedpoint.Abs(one.Low.Sub(one.Close).Div(one.Close)) > limit {
							if fixedpoint.Abs(one.Open.Sub(one.High).Div(one.High)) > limit {
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

func (s *DojiLongLeggedStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

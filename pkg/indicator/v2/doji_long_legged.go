package indicatorv2

import (
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
func DojiLongLegged(source KLineSubscription) *DojiLongLeggedStream {
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
		if four.Close.Float64() > four.Open.Float64() {
			if three.Close.Float64() > three.Open.Float64() {
				if two.Close.Float64() > two.Open.Float64() {
					if one.Close.Sub(one.Open).Div(one.Open).Abs().Float64() < threshold {
						if one.High.Sub(one.Open).Div(one.Open).Abs().Float64() > limit {
							if one.Close.Sub(one.Low).Div(one.Low).Abs().Float64() > limit {
								output = Bear
							}
						}
					}
				}
			}
		}

		// BULL
		if four.Close.Float64() < four.Open.Float64() {
			if three.Close.Float64() < three.Open.Float64() {
				if two.Close.Float64() < two.Open.Float64() {
					if one.Open.Sub(one.Close).Div(one.Close).Abs().Float64() < threshold {
						if one.Low.Sub(one.Close).Div(one.Close).Abs().Float64() > limit {
							if one.Open.Sub(one.High).Div(one.High).Abs().Float64() > limit {
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

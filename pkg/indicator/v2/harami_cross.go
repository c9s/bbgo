package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"

	"github.com/c9s/bbgo/pkg/types"
)

type HaramiCrossStream struct {
	*types.Float64Series

	window int
}

// A bullish harami cross pattern forms after a downtrend.
// The doji shows that some indecision has entered the minds of sellers.
// Typically, traders don't act on the pattern unless the price follows through to the upside within the next
// couple of candles. This is called confirmation. Sometimes the price may pause for a few candles after the doji,
// and then rise or fall. A rise above the open of the first candle helps confirm that the price may be heading higher.
// A bearish harami cross forms after an uptrend.
// The first candlestick is a long up candle (typically colored white or green) which shows buyers are in control.
// This is followed by a doji, which shows indecision on the part of the buyers. Once again, the doji
// must be contained within the real body of the prior candle.
// If the price drops following the pattern, this confirms the pattern.
// If the price continues to rise following the doji, the bearish pattern is invalidated.
func HaramiCross(source KLineSubscription, direction Direction, maxDiff float64) *HaramiCrossStream {
	s := &HaramiCrossStream{
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
			two        = source.Last(1)
			one        = source.Last(0)
			isLastDoji = fixedpoint.ApproxEqual(one.Open, one.Close, maxDiff)
		)
		if direction == Bullish {
			var (
				isBullishHaramiCrossPattern = two.Open > one.Open &&
					two.Close < one.Open &&
					two.Close < one.Close &&
					two.Open > one.Low &&
					two.High > one.High
			)
			if isBullishHaramiCrossPattern && isLastDoji {
				output = Bull
			}
		} else {
			var isBearishHaramiCrossPattern = two.Open < one.Open &&
				two.Close > one.Open &&
				two.Close > one.Close &&
				two.Open < one.Low &&
				two.High > one.High
			if isBearishHaramiCrossPattern && isLastDoji {
				output = Bear
			}
		}

		s.PushAndEmit(output)

	})

	return s
}

func (s *HaramiCrossStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

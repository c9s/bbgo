package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type SpinningTopStream struct {
	*types.Float64Series
}

// A spinning top is a candlestick pattern that has a short real body that's vertically centered
// between long upper and lower shadows.
// The real body should be small, showing little difference between the open and close prices.
// Since buyers and sellers both pushed the price, but couldn't maintain it, the pattern shows
// indecision. More sideways movement could follow.
func SpinningTop(source KLineSubscription, direction Direction) *SpinningTopStream {
	s := &SpinningTopStream{
		Float64Series: types.NewFloat64Series(),
	}

	source.AddSubscriber(func(kLine types.KLine) {
		var (
			output     = Neutral
			one        = source.Last(0)
			bodyLength = one.Close.Sub(one.Open).Abs().Float64()
		)

		var upperShadowLength, lowerShadowLength float64
		if direction == Bullish {
			// a bullish candle closes above its open, so the body sits between open and close
			upperShadowLength = one.High.Sub(one.Close).Abs().Float64()
			lowerShadowLength = one.Open.Sub(one.Low).Abs().Float64()
		} else {
			// a bearish candle closes below its open
			upperShadowLength = one.High.Sub(one.Open).Abs().Float64()
			lowerShadowLength = one.Close.Sub(one.Low).Abs().Float64()
		}

		if bodyLength < upperShadowLength && bodyLength < lowerShadowLength {
			if direction == Bullish {
				output = Bull
			} else {
				output = Bear
			}
		}

		s.PushAndEmit(output)
	})

	return s
}

func (s *SpinningTopStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

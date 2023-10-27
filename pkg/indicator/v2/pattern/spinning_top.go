package pattern

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
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
func SpinningTop(source v2.KLineSubscription, direction Direction) *SpinningTopStream {
	s := &SpinningTopStream{
		Float64Series: types.NewFloat64Series(),
	}

	source.AddSubscriber(func(kLine types.KLine) {
		var (
			output     = Neutral
			one        = source.Last(0)
			bodyLength = fixedpoint.Abs(one.Close - one.Open)
		)
		if direction == Bullish {
			var (
				upperShadowLength = fixedpoint.Abs(one.High - one.Close)
				lowerShadowLength = fixedpoint.Abs(one.Open - one.Low)
				isSpinningTop     = bodyLength < upperShadowLength &&
					bodyLength < lowerShadowLength
			)

			if isSpinningTop {
				output = Bull
			}
		} else {
			var (
				upperShadowLength    = fixedpoint.Abs(one.High - one.Open)
				lowerShadowLength    = fixedpoint.Abs(one.High - one.Low)
				isBearishSpinningTop = bodyLength < upperShadowLength &&
					bodyLength < lowerShadowLength
			)
			if isBearishSpinningTop {
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

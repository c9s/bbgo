package pattern

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type DojiGraveStoneStream struct {
	*types.Float64Series
}

// A gravestone doji candle is a pattern that technical stock traders use as a signal that a stock price
// may soon undergo a bearish reversal. This pattern forms when the open, low, and closing prices of an asset
// are close to each other and have a long upper shadow. The shadow in a candlestick chart is the thin part
// showing the price action for the day as it differs from high to low prices.
func DojiGraveStone(source v2.KLineSubscription, maxDiff float64) *DojiGraveStoneStream {
	s := &DojiGraveStoneStream{
		Float64Series: types.NewFloat64Series(),
	}

	source.AddSubscriber(func(kLine types.KLine) {
		var (
			output         = Neutral
			one            = kLine
			openEqualClose = fixedpoint.ApproxEqual(one.Open, one.Close, maxDiff)
			highEqualsOpen = fixedpoint.ApproxEqual(one.Open, one.High, maxDiff)
			lowEqualsClose = fixedpoint.ApproxEqual(one.Close, one.Low, maxDiff)
		)

		if openEqualClose && lowEqualsClose && !highEqualsOpen {
			output = Bear
		}

		s.PushAndEmit(output)

	})

	return s
}

func (s *DojiGraveStoneStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

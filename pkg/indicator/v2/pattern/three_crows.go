package pattern

import (
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// https://www.candlescanner.com/candlestick-patterns/two-crows/
// The Two Crows is a three-line bearish reversal candlestick pattern.
// The pattern requires confirmation, that is, the following candles should break
// a trendline or the nearest support area which may be formed by the first candle’s line.
// If the pattern is not confirmed it may act only as a temporary pause within an uptrend.
// Although the pattern name suggest that two lines form it, in fact, it contains three lines
type ThreeCrowsStream struct {
	*types.Float64Series

	window int
}

func ThreeCrows(source v2.KLineSubscription) *ThreeCrowsStream {
	s := &ThreeCrowsStream{
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
			three        = source.Last(2)
			two          = source.Last(1)
			one          = source.Last(0)
			isDownTrend  = three.Low > two.Low && two.Low > one.Low
			isAllBearish = three.Open > three.Close &&
				two.Open > two.Close && one.Open > one.Close
			opensWithinPreviousBody = three.Open > two.Open &&
				two.Open > three.Close &&
				two.Open > one.Open &&
				one.Open > two.Close
		)

		if isDownTrend && isAllBearish && opensWithinPreviousBody {
			output = Bear
		}

		s.PushAndEmit(output)

	})

	return s
}

func (s *ThreeCrowsStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

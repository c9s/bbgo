package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type ThreeCrowsStream struct {
	*types.Float64Series

	window int
}

// https://www.candlescanner.com/candlestick-patterns/two-crows/
// The Two Crows is a three-line bearish reversal candlestick pattern.
// The pattern requires confirmation, that is, the following candles should break
// a trendline or the nearest support area which may be formed by the first candle’s line.
// If the pattern is not confirmed it may act only as a temporary pause within an uptrend.
// Although the pattern name suggest that two lines form it, in fact, it contains three lines
func ThreeCrows(source KLineSubscription) *ThreeCrowsStream {
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
			three       = source.Last(2)
			two         = source.Last(1)
			one         = source.Last(0)
			isDownTrend = three.Low.Float64() > two.Low.Float64() &&
				two.Low.Float64() > one.Low.Float64()
			isAllBearish = three.Open.Float64() > three.Close.Float64() &&
				two.Open.Float64() > two.Close.Float64() &&
				one.Open.Float64() > one.Close.Float64()
			opensWithinPreviousBody = three.Open.Float64() > two.Open.Float64() &&
				two.Open.Float64() > three.Close.Float64() &&
				two.Open.Float64() > one.Open.Float64() &&
				one.Open.Float64() > two.Close.Float64()
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

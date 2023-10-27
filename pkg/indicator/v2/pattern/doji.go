package pattern

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type DojiStream struct {
	*types.Float64Series
}

// Is a doji bullish or bearish?
// A doji formation generally can be interpreted as a sign of indecision, meaning neither bulls nor bears
// can successfully take over. Of its variations, the dragonfly doji is seen as a bullish reversal pattern
// that occurs at the bottom of downtrends. The gravestone doji is read as a bearish reversal at the peak of uptrends.
func Doji(source v2.KLineSubscription, maxDiff float64) *DojiStream {
	s := &DojiStream{
		Float64Series: types.NewFloat64Series(),
	}

	source.AddSubscriber(func(kLine types.KLine) {
		var (
			output         = Neutral
			one            = source.Last(0)
			openEqualClose = fixedpoint.ApproxEqual(one.Open, one.Close, maxDiff)
			highEqualsOpen = fixedpoint.ApproxEqual(one.Open, one.High, maxDiff)
			lowEqualsClose = fixedpoint.ApproxEqual(one.Close, one.Low, maxDiff)
		)
		if openEqualClose && lowEqualsClose && highEqualsOpen {
			output = Bull
		}
		s.PushAndEmit(output)
	})

	return s
}

func (s *DojiStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

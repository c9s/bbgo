package pattern

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type DojiGraveStoneStream struct {
	*types.Float64Series
}

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

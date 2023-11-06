package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type DojiDragonFlyStream struct {
	*types.Float64Series
}

func DojiDragonFly(source KLineSubscription, maxDiff float64) *DojiDragonFlyStream {
	s := &DojiDragonFlyStream{
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

		if openEqualClose && highEqualsOpen && !lowEqualsClose {
			output = Bull
		}
		s.PushAndEmit(output)

	})

	return s
}

func (s *DojiDragonFlyStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

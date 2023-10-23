package pattern

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type DojiDragonFlyStream struct {
	*types.Float64Series

	window int
}

func DojiDragonFly(source v2.KLineSubscription) *DojiDragonFlyStream {
	s := &DojiDragonFlyStream{
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
			one            = source.Last(0)
			openEqualClose = fixedpoint.ApproxEqual(one.Open, one.Close, 0.001)
			highEqualsOpen = fixedpoint.ApproxEqual(one.Open, one.High, 0.001)
			lowEqualsClose = fixedpoint.ApproxEqual(one.Close, one.Low, 0.001)
		)

		if openEqualClose && highEqualsOpen && !lowEqualsClose {
			output = Bull
		}
		s.PushAndEmit(output)

	})

	return s
}

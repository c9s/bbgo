package pattern

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type MarubozuStream struct {
	*types.Float64Series

	window int
}

func Marubozu(source v2.KLineSubscription, maxDiff float64) *MarubozuStream {
	s := &MarubozuStream{
		Float64Series: types.NewFloat64Series(),
		window:        2,
	}

	source.AddSubscriber(func(kLine types.KLine) {
		var (
			output = Neutral
			one    = source.Last(0)
		)
		// BEAR
		if one.Open > one.Close {
			if fixedpoint.ApproxEqual(one.High, one.Open, maxDiff) &&
				fixedpoint.ApproxEqual(one.Low, one.Close, maxDiff) {
				output = Bear
			}
		}

		// BULL
		if one.Open < one.Close {
			if fixedpoint.ApproxEqual(one.Low, one.Open, maxDiff) &&
				fixedpoint.ApproxEqual(one.High, one.Close, maxDiff) {
				output = Bull
			}
		}

		s.PushAndEmit(output)

	})

	return s
}

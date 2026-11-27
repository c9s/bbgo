package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"

	"github.com/c9s/bbgo/pkg/types"
)

type MarubozuStream struct {
	*types.Float64Series

	window int
}

func Marubozu(source KLineSubscription, maxDiff float64) *MarubozuStream {
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
		if one.Open.Float64() > one.Close.Float64() {
			if fixedpoint.ApproxEqual(one.High, one.Open, maxDiff) &&
				fixedpoint.ApproxEqual(one.Low, one.Close, maxDiff) {
				output = Bear
			}
		}

		// BULL
		if one.Open.Float64() < one.Close.Float64() {
			if fixedpoint.ApproxEqual(one.Low, one.Open, maxDiff) &&
				fixedpoint.ApproxEqual(one.High, one.Close, maxDiff) {
				output = Bull
			}
		}

		s.PushAndEmit(output)

	})

	return s
}

func (s *MarubozuStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

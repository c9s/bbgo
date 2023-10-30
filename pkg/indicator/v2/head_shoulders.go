package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type HeadShoulderStream struct {
	*types.Float64Series

	max    *MaxValueStream
	min    *MinValueStream
	window int
}

// Basic Head-Shoulder Detection
func HeadShoulderSimple(source KLineSubscription) *HeadShoulderStream {
	var (
		window = 3
		high   = HighPrices(source)
		low    = LowPrices(source)
		s      = &HeadShoulderStream{
			Float64Series: types.NewFloat64Series(),
			max:           MaxValue(high, window),
			min:           MinValue(low, window),
			window:        window,
		}
	)

	source.AddSubscriber(func(kLine types.KLine) {
		var (
			i      = source.Length()
			output = Neutral
		)
		if i < s.window {
			s.PushAndEmit(output)
			return
		}
		if s.min.Last(1) < low.Last(2) &&
			s.min.Last(1) < low.Last(0) &&
			low.Last(1) > low.Last(2) &&
			low.Last(1) > low.Last(0) {
			output = Bull // inverse head/shoulder pattern
		} else if s.max.Last(1) > high.Last(2) &&
			s.max.Last(1) > high.Last(0) &&
			high.Last(1) < high.Last(2) &&
			high.Last(1) < high.Last(0) {
			output = Bear // head/shoulder pattern
		}

		s.PushAndEmit(output)
	})

	return s
}

func (s *HeadShoulderStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

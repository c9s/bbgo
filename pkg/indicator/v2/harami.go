package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type HaramiStream struct {
	*types.Float64Series

	window int
}

// The bullish harami indicator is charted as a long candlestick followed by a smaller body,
// referred to as a doji, that is completely contained within the vertical range of the previous body.
// To some, a line drawn around this pattern resembles a pregnant woman. The word harami comes from an
// old Japanese word meaning pregnant.
func Harami(source KLineSubscription) *HaramiStream {
	s := &HaramiStream{
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
			two = source.Last(1)
			one = source.Last(0)
		)

		// BEAR
		if two.Open.Float64() < two.Close.Float64() {
			if one.Open.Float64() > one.Close.Float64() {
				if one.Open.Float64() < two.Close.Float64() && one.Close.Float64() > two.Open.Float64() {
					output = Bear
				}
			}
		}

		// BULL
		if two.Open.Float64() > two.Close.Float64() {
			if one.Open.Float64() < one.Close.Float64() {
				if one.Open.Float64() > two.Close.Float64() && one.Close.Float64() < two.Open.Float64() {
					output = Bull
				}
			}
		}
		s.PushAndEmit(output)

	})

	return s
}

func (s *HaramiStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type ThreeLineStrikeStream struct {
	*types.Float64Series

	window int
}

func ThreeLineStrike(source KLineSubscription) *ThreeLineStrikeStream {
	s := &ThreeLineStrikeStream{
		Float64Series: types.NewFloat64Series(),
		window:        4,
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
			four  = source.Last(3)
			three = source.Last(2)
			two   = source.Last(1)
			one   = source.Last(0)
		)
		// BEAR
		if three.Close.Float64() < four.Close.Float64() {
			if two.Close.Float64() < three.Close.Float64() {
				if four.Close.Float64() < three.Open.Float64() &&
					three.Open.Float64() < four.Open.Float64() {
					if three.Close.Float64() < two.Open.Float64() &&
						two.Open.Float64() < three.Open.Float64() {
						if one.Open.Float64() < two.Close.Float64() {
							if one.Close.Float64() > four.Open.Float64() {
								output = Bear
							}
						}
					}
				}
			}
		}
		// BULL
		if three.Close.Float64() > four.Close.Float64() {
			if two.Close.Float64() > three.Close.Float64() {
				if four.Close.Float64() > three.Open.Float64() &&
					three.Open.Float64() > four.Open.Float64() {
					if three.Close.Float64() > two.Open.Float64() &&
						two.Open.Float64() > three.Open.Float64() {
						if one.Open.Float64() > two.Close.Float64() {
							if one.Close.Float64() < four.Open.Float64() {
								output = Bull
							}
						}
					}
				}
			}
		}

		s.PushAndEmit(output)
	})

	return s
}

func (s *ThreeLineStrikeStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

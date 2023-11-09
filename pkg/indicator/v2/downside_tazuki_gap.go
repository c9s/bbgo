package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type TazukiGapStream struct {
	*types.Float64Series

	window int
}

func TazukiGap(source KLineSubscription) *TazukiGapStream {
	s := &TazukiGapStream{
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
			three            = source.Last(2)
			two              = source.Last(1)
			one              = source.Last(0)
			isFirstBearish   = three.Close.Float64() < three.Open.Float64()
			isSecondBearish  = two.Close.Float64() < two.Open.Float64()
			isThirdBullish   = one.Close.Float64() > one.Open.Float64()
			isFirstGapExists = two.High.Float64() < three.Low.Float64()
			isTazukiGap      = two.Open.Float64() > one.Open.Float64() &&
				two.Close.Float64() < one.Open.Float64() &&
				one.Close.Float64() > two.Open.Float64() &&
				one.Close.Float64() < three.Close.Float64()
		)

		if isFirstBearish && isSecondBearish && isThirdBullish && isFirstGapExists && isTazukiGap {
			output = Bear
		}

		s.PushAndEmit(output)

	})

	return s
}

func (s *TazukiGapStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

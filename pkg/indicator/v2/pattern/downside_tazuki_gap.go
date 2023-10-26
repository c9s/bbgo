package pattern

import (
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type TazukiGapStream struct {
	*types.Float64Series

	window int
}

func TazukiGap(source v2.KLineSubscription) *TazukiGapStream {
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
			isFirstBearish   = three.Close < three.Open
			isSecondBearish  = two.Close < two.Open
			isThirdBullish   = one.Close > one.Open
			isFirstGapExists = two.High < three.Low
			isTazukiGap      = two.Open > one.Open &&
				two.Close < one.Open &&
				one.Close > two.Open &&
				one.Close < three.Close
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

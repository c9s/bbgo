package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type PiercingLineStream struct {
	*types.Float64Series

	window int
}

func PiercingLine(source KLineSubscription) *PiercingLineStream {
	s := &PiercingLineStream{
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
			two             = source.Last(1)
			one             = source.Last(0)
			firstMidpoint   = two.Open.Add(two.Close).Div(fixedpoint.Two).Float64()
			isDowntrend     = one.Low.Float64() < two.Low.Float64()
			isFirstBearish  = two.Close.Float64() < two.Open.Float64()
			isSecondBullish = one.Close.Float64() > one.Open.Float64()
			isPiercingLine  = two.Low.Float64() > one.Open.Float64() &&
				one.Close.Float64() > firstMidpoint
		)
		if isDowntrend && isFirstBearish && isSecondBullish && isPiercingLine {
			output = Bull
		}

		s.PushAndEmit(output)

	})

	return s
}

func (s *PiercingLineStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}

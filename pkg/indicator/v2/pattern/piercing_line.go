package pattern

import (
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type PiercingLineStream struct {
	*types.Float64Series

	window int
}

func PiercingLine(source v2.KLineSubscription) *PiercingLineStream {
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
			firstMidpoint   = (two.Open + two.Close) / 2
			isDowntrend     = one.Close < two.Low
			isFirstBearish  = two.Close < two.Open
			isSecondBullish = one.Close > one.Open
			isPiercingLine  = two.Low > one.Open &&
				one.Close > firstMidpoint
		)
		if isDowntrend && isFirstBearish && isSecondBullish && isPiercingLine {
			output = Bull
		}

		s.PushAndEmit(output)

	})

	return s
}

package pattern

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type SpinningTopStream struct {
	*types.Float64Series
}

func SpinningTop(source v2.KLineSubscription) *SpinningTopStream {
	s := &SpinningTopStream{
		Float64Series: types.NewFloat64Series(),
	}

	source.AddSubscriber(func(kLine types.KLine) {
		var (
			output            = Neutral
			one               = source.Last(0)
			bodyLength        = fixedpoint.Abs(one.Close - one.Open)
			upperShadowLength = fixedpoint.Abs(one.High - one.Close)
			lowerShadowLength = fixedpoint.Abs(one.Open - one.Low)
			isSpinningTop     = bodyLength < upperShadowLength &&
				bodyLength < lowerShadowLength
		)

		if isSpinningTop {
			output = Bull
		} else {
			upperShadowLength = fixedpoint.Abs(one.High - one.Open)
			lowerShadowLength = fixedpoint.Abs(one.High - one.Low)
			var isBearishSpinningTop = bodyLength < upperShadowLength &&
				bodyLength < lowerShadowLength
			if isBearishSpinningTop {
				output = Bear
			}
		}

		s.PushAndEmit(output)

	})

	return s
}

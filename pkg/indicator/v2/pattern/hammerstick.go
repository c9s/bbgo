package pattern

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type HammerStickStream struct {
	*types.Float64Series
	inverted bool
	window   int
}

func HammerStick(source v2.KLineSubscription, multiplier float64, inverted ...bool) *HammerStickStream {
	var i bool
	if len(inverted) > 0 {
		i = true
	}

	s := &HammerStickStream{
		Float64Series: types.NewFloat64Series(),
		window:        2,
		inverted:      i,
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
			one             = source.Last(0)
			oc              = one.Open - one.Close
			co              = one.Close - one.Open
			isBearishHammer = one.Open > one.Close &&
				fixedpoint.ApproxEqual(one.Open, one.High, multiplier) &&
				oc >= 2*(one.Close-one.Low)
			isBearishInvertedHammer = one.Open > one.Close &&
				fixedpoint.ApproxEqual(one.Close, one.Low, multiplier) &&
				oc <= 2*(one.High-one.Open)
			isBullishHammer = one.Close > one.Open &&
				fixedpoint.ApproxEqual(one.Close, one.High, multiplier) &&
				co <= 2*(one.Open-one.Low)
			isBullishInvertedHammer = one.Close > one.Open &&
				fixedpoint.ApproxEqual(one.Open, one.Low, multiplier) &&
				co <= 2*(one.High-one.Close)
		)

		if !s.inverted && isBearishHammer || s.inverted && isBearishInvertedHammer {
			output = Bear
		} else if !s.inverted && isBullishHammer || s.inverted && isBullishInvertedHammer {
			output = Bull
		}

		s.PushAndEmit(output)

	})

	return s
}

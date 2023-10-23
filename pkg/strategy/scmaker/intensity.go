package scmaker

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/indicator/v2/trend"
	"github.com/c9s/bbgo/pkg/types"
)

type IntensityStream struct {
	*types.Float64Series

	Buy, Sell *trend.RMAStream
	window    int
}

func Intensity(source indicatorv2.KLineSubscription, window int) *IntensityStream {
	s := &IntensityStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,

		Buy:  trend.RMA2(types.NewFloat64Series(), window, false),
		Sell: trend.RMA2(types.NewFloat64Series(), window, false),
	}

	threshold := fixedpoint.NewFromFloat(100.0)
	source.AddSubscriber(func(k types.KLine) {
		volume := k.Volume.Float64()

		// ignore zero volume events or <= 10usd events
		if volume == 0.0 || k.Close.Mul(k.Volume).Compare(threshold) <= 0 {
			return
		}

		c := k.Close.Compare(k.Open)
		if c > 0 {
			s.Buy.PushAndEmit(volume)
		} else if c < 0 {
			s.Sell.PushAndEmit(volume)
		}
		s.Float64Series.PushAndEmit(k.High.Sub(k.Low).Float64())
	})

	return s
}

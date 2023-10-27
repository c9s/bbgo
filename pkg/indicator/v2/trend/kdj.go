package trend

import (
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type KDJStream struct {
	*types.Float64Series
	K      *SMAStream
	D      *SMAStream
	min    *v2.MinValueStream
	max    *v2.MaxValueStream
	window int
}

// The Kdj function calculates the KDJ  indicator, also known as
// the Random Index. KDJ is calculated similar to the Stochastic
// Oscillator with the difference of having the J line. It is
// used to analyze the trend and entry points.
//
// The K and D lines show if the asset is overbought when they
// crosses above 80%, and oversold when they crosses below
// 20%. The J line represents the divergence.
//
// RSV = ((Closing - Min(Low, rPeriod))
//
//	/ (Max(High, rPeriod) - Min(Low, rPeriod))) * 100
//
// K = Sma(RSV, kPeriod)
// D = Sma(K, dPeriod)
// J = (3 * K) - (2 * D)
func KDJ(source v2.KLineSubscription, window, kWindow, dWindow int) *KDJStream {

	s := &KDJStream{
		Float64Series: types.NewFloat64Series(),
		min:           v2.MinValue(v2.LowPrices(source), window),
		max:           v2.MaxValue(v2.HighPrices(source), window),
		K:             SMA(nil, kWindow),
		D:             SMA(nil, dWindow),
		window:        window,
	}
	source.AddSubscriber(func(v types.KLine) {
		var (
			closing = v.Close.Float64()
			highest = s.max.Last(0)
			lowest  = s.min.Last(0)
			rsv     = (closing - lowest) / (highest - lowest) * 100
			k       = s.K.Calculate(rsv)
			d       = s.D.Calculate(k)
			j       = (3 * k) - (2 * d)
		)
		s.PushAndEmit(j)
	})

	return s
}

// The DefaultKdj function calculates KDJ based on default periods
// consisting of window of 9, kPeriod of 3, and dPeriod of 3.
func KDJDefault(source v2.KLineSubscription) *KDJStream {
	return KDJ(source, 9, 3, 3)
}

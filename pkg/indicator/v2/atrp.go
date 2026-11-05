package indicatorv2

import "github.com/c9s/bbgo/pkg/types"

// ATRPStream is the Average True Range Percent: ATR / close * 100
type ATRPStream struct {
	*types.Float64Series
}

func ATRP2(source KLineSubscription, window int) *ATRPStream {
	s := &ATRPStream{
		Float64Series: types.NewFloat64Series(),
	}
	tr := TR2(source)
	atr := RMA2(tr, window, true)
	atr.AddSubscriber(func(x float64) {
		// x is the last rma
		k := source.Last(0)
		cloze := k.Close.Float64()
		atrp := x / cloze * 100
		s.PushAndEmit(atrp)
		s.Slice = generalTruncate(s.Slice)
	})
	return s
}

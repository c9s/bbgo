package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// OBVStream implements the on-balance volume indicator
//
// On-Balance Volume (OBV) Definition
// - https://www.investopedia.com/terms/o/onbalancevolume.asp
//
// On-Balance Volume (OBV) is a technical analysis indicator that uses volume information to predict changes in stock price.
// The idea behind OBV is that volume precedes price: when the OBV is rising, it means that buyers are becoming more aggressive and
// that the stock price is likely to follow suit. When the OBV is falling, it indicates that sellers are becoming more aggressive and
// that the stock price is likely to decrease. OBV is calculated by adding the volume on days when the stock price closes higher and
// subtracting the volume on days when the stock price closes lower. This running total forms the OBV line, which can then be used
// to make predictions about future stock price movements.
//
// The first kline seeds the OBV with its volume. A kline that closes at the
// same price as the previous one leaves the OBV unchanged.
type OBVStream struct {
	*types.Float64Series

	prevClose float64
}

func OBV(source KLineSubscription) *OBVStream {
	s := &OBVStream{
		Float64Series: types.NewFloat64Series(),
	}

	source.AddSubscriber(func(k types.KLine) {
		s.PushAndEmit(s.calculate(k.Close.Float64(), k.Volume.Float64()))
		s.Truncate()
	})

	return s
}

func (s *OBVStream) calculate(price, volume float64) float64 {
	if s.Length() == 0 {
		s.prevClose = price
		return volume
	}

	prev := s.prevClose
	s.prevClose = price

	switch {
	case price > prev:
		return s.Last(0) + volume
	case price < prev:
		return s.Last(0) - volume
	default:
		return s.Last(0)
	}
}

func (s *OBVStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}

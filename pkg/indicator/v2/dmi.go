package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// DMIStream implements the Directional Movement Index
//
// Refer: https://www.investopedia.com/terms/d/dmi.asp
// Refer: https://github.com/twopirllc/pandas-ta/blob/main/pandas_ta/trend/adx.py
//
// The Directional Movement Index (DMI) is a technical analysis indicator that is used to identify the direction and strength of a trend
// in a security's price. It was developed by J. Welles Wilder and is based on the concept of the +DI and -DI lines, which measure the strength
// of upward and downward price movements, respectively. The DMI is calculated by taking the difference between the +DI and -DI lines, and then
// smoothing the result using a moving average. This resulting line is called the Average Directional Index (ADX), and is used to identify whether
// a security is trending or not. If the ADX is above a certain threshold, typically 20, it indicates that the security is in a strong trend,
// and if it is below that threshold it indicates that the security is in a sideways or choppy market. The DMI can be used by traders to confirm
// the direction and strength of a trend, or to identify potential entry and exit points for trades.
//
// The embedded RMAStream is the ADX line (smoothed with adxSmoothing); DIPlus and
// DIMinus are the +DI and -DI lines (smoothed with window). Unlike ADXStream, the
// DI and DX values are only produced once the ATR has `window` values, and the ADX
// can be smoothed with a separate window, matching the legacy DMI indicator.
// DIPlus and DIMinus are pushed before the ADX value is emitted.
type DMIStream struct {
	*RMAStream

	DIPlus, DIMinus *types.Float64Series

	atr      *ATRStream
	dmp, dmn *RMAStream

	window            int
	initialized       bool
	prevHigh, prevLow float64
}

// DMI creates a DMI stream. window is the +DI/-DI (and ATR) smoothing window,
// adxSmoothing is the ADX smoothing window.
func DMI(source KLineSubscription, window, adxSmoothing int) *DMIStream {
	checkWindow(window)
	checkWindow(adxSmoothing)

	var (
		atr = ATR2(source, window)
		pos = types.NewFloat64Series()
		neg = types.NewFloat64Series()
		dx  = types.NewFloat64Series()
	)

	s := &DMIStream{
		RMAStream: RMA2(dx, adxSmoothing, true),
		DIPlus:    types.NewFloat64Series(),
		DIMinus:   types.NewFloat64Series(),
		atr:       atr,
		dmp:       RMA2(pos, window, true),
		dmn:       RMA2(neg, window, true),
		window:    window,
	}

	source.AddSubscriber(func(k types.KLine) {
		high, low := k.High.Float64(), k.Low.Float64()
		if !s.initialized {
			s.initialized = true
			s.prevHigh, s.prevLow = high, low
			return
		}

		up := high - s.prevHigh
		dn := s.prevLow - low
		s.prevHigh, s.prevLow = high, low

		p := 0.0
		if up > dn && up > 0. {
			p = up
		}

		n := 0.0
		if dn > up && dn > 0. {
			n = dn
		}

		pos.PushAndEmit(p)
		neg.PushAndEmit(n)
		pos.Slice = generalTruncate(pos.Slice)
		neg.Slice = generalTruncate(neg.Slice)

		if s.atr.Length() < s.window {
			return
		}

		dmp := s.dmp.Last(0)
		dmn := s.dmn.Last(0)
		atrValue := s.atr.Last(0)

		// on intervals without volatility (e.g. flat candles without trades) ATR and
		// both directional movements are zero; gate the divisions so the smoothed
		// state never turns NaN or Inf, which would poison the RMA permanently
		diPlus, diMinus := 0., 0.
		if atrValue != 0 {
			r := 100. / atrValue
			diPlus = r * dmp
			diMinus = r * dmn
		}

		s.DIPlus.PushAndEmit(diPlus)
		s.DIMinus.PushAndEmit(diMinus)
		s.DIPlus.Slice = generalTruncate(s.DIPlus.Slice)
		s.DIMinus.Slice = generalTruncate(s.DIMinus.Slice)

		v := 0.
		if dmSum := dmp + dmn; dmSum != 0 {
			v = 100. * math.Abs(dmp-dmn) / dmSum
		}
		dx.PushAndEmit(v)
		dx.Slice = generalTruncate(dx.Slice)
	})

	return s
}

// GetADX returns the ADX series (the embedded RMAStream)
func (s *DMIStream) GetADX() *RMAStream {
	return s.RMAStream
}

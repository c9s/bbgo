package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// KlingerOscillatorStream is the Klinger Oscillator
//
// Refer: Klinger Oscillator
// Refer URL: https://www.investopedia.com/terms/k/klingeroscillator.asp
// Explanation:
// The Klinger Oscillator is a technical indicator that was developed by Stephen Klinger.
// It is based on the assumption that there is a relationship between money flow and price movement in the stock market.
// The Klinger Oscillator is calculated by taking the difference between a 34-period and 55-period moving average.
// Usually the indicator is using together with a 9-period or 13-period of moving average as the signal line.
// This indicator is often used to identify potential turning points in the market, as well as to confirm the strength of a trend.
//
//	KO = EMA(VF, fastWindow) - EMA(VF, slowWindow)
//
// The volume force (VF) of the first kline is the raw volume and is not fed into the moving averages,
// so the oscillator emits its first value on the second kline.
type KlingerOscillatorStream struct {
	*types.Float64Series

	fastWindow, slowWindow int

	// VolumeForce is the volume force series, it has one value for each kline (including the first one)
	VolumeForce *types.Float64Series

	Fast, Slow *EWMAStream

	// volume force calculation state
	dm, cm, trend, lastSum float64
}

// KlingerOscillator creates the Klinger Oscillator stream.
// When fastWindow or slowWindow is 0, the default 34 and 55 are used respectively.
func KlingerOscillator(source KLineSubscription, fastWindow, slowWindow int) *KlingerOscillatorStream {
	if fastWindow == 0 {
		fastWindow = 34
	}
	if slowWindow == 0 {
		slowWindow = 55
	}

	s := &KlingerOscillatorStream{
		Float64Series: types.NewFloat64Series(),
		fastWindow:    fastWindow,
		slowWindow:    slowWindow,
		VolumeForce:   types.NewFloat64Series(),
		// the inner EWMA streams are driven by the kline subscriber
		Fast: EWMA2(nil, fastWindow),
		Slow: EWMA2(nil, slowWindow),
	}

	if source != nil {
		source.AddSubscriber(func(k types.KLine) {
			if v, ok := s.calculate(k.High.Float64(), k.Low.Float64(), k.Close.Float64(), k.Volume.Float64()); ok {
				s.PushAndEmit(v)
				s.Truncate()
			}
		})
	}
	return s
}

func (s *KlingerOscillatorStream) calculate(high, low, cloze, volume float64) (float64, bool) {
	initialized := s.lastSum > 0

	vf := s.updateVolumeForce(high, low, cloze, volume)
	s.VolumeForce.PushAndEmit(vf)
	s.VolumeForce.Slice = generalTruncate(s.VolumeForce.Slice)

	if !initialized {
		return 0, false
	}

	s.Fast.PushAndEmit(s.Fast.Calculate(vf))
	s.Fast.Truncate()
	s.Slow.PushAndEmit(s.Slow.Calculate(vf))
	s.Slow.Truncate()
	return s.Fast.Last(0) - s.Slow.Last(0), true
}

func (s *KlingerOscillatorStream) updateVolumeForce(high, low, cloze, volume float64) float64 {
	sum := high + low + cloze
	if s.lastSum == 0 {
		s.dm = high - low
		s.cm = s.dm
		s.trend = 1.
		s.lastSum = sum
		return volume // first volume is not calculated
	}

	trend := 1.
	if sum <= s.lastSum {
		trend = -1.
	}

	dm := high - low
	if s.trend == trend {
		s.cm = s.cm + dm
	} else {
		s.cm = s.dm + dm
	}

	s.trend = trend
	s.lastSum = sum
	s.dm = dm
	return volume * (2.*(s.dm/s.cm) - 1.) * trend
}

func (s *KlingerOscillatorStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}

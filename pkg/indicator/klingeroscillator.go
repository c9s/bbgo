package indicator

import (
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Klinger Oscillator
// Refer URL: https://www.investopedia.com/terms/k/klingeroscillator.asp
// Explanation:
// The Klinger Oscillator is a technical indicator that was developed by Stephen Klinger.
// It is based on the assumption that there is a relationship between money flow and price movement in the stock market.
// The Klinger Oscillator is calculated by taking the difference between a 34-period and 55-period moving average.
// Usually the indicator is using together with a 9-period or 13-period of moving average as the signal line.
// This indicator is often used to identify potential turning points in the market, as well as to confirm the strength of a trend.
//
//go:generate callbackgen -type KlingerOscillator
type KlingerOscillator struct {
	types.SeriesBase
	types.IntervalWindow
	Fast types.UpdatableSeries
	Slow types.UpdatableSeries
	VF   VolumeForce

	updateCallbacks []func(value float64)
}

func (inc *KlingerOscillator) Length() int {
	if inc.Fast == nil || inc.Slow == nil {
		return 0
	}
	return inc.Fast.Length()
}

func (inc *KlingerOscillator) Last(i int) float64 {
	if inc.Fast == nil || inc.Slow == nil {
		return 0
	}
	return inc.Fast.Last(i) - inc.Slow.Last(i)
}

func (inc *KlingerOscillator) Update(high, low, cloze, volume float64) {
	if inc.Fast == nil {
		inc.SeriesBase.Series = inc
		inc.Fast = &EWMA{IntervalWindow: types.IntervalWindow{Window: 34, Interval: inc.Interval}}
		inc.Slow = &EWMA{IntervalWindow: types.IntervalWindow{Window: 55, Interval: inc.Interval}}
	}

	if inc.VF.lastSum > 0 {
		inc.VF.Update(high, low, cloze, volume)
		inc.Fast.Update(inc.VF.Value)
		inc.Slow.Update(inc.VF.Value)
	} else {
		inc.VF.Update(high, low, cloze, volume)
	}
}

var _ types.SeriesExtend = &KlingerOscillator{}

func (inc *KlingerOscillator) PushK(k types.KLine) {
	inc.Update(k.High.Float64(), k.Low.Float64(), k.Close.Float64(), k.Volume.Float64())
}

func (inc *KlingerOscillator) BindK(target KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}

// Utility to hold the state of calculation
type VolumeForce struct {
	dm      float64
	cm      float64
	trend   float64
	lastSum float64
	Value   float64
}

func (inc *VolumeForce) Update(high, low, cloze, volume float64) {
	if inc.lastSum == 0 {
		inc.dm = high - low
		inc.cm = inc.dm
		inc.trend = 1.
		inc.lastSum = high + low + cloze
		inc.Value = volume // first volume is not calculated
		return
	}
	trend := 1.
	if high+low+cloze <= inc.lastSum {
		trend = -1.
	}
	dm := high - low
	if inc.trend == trend {
		inc.cm = inc.cm + dm
	} else {
		inc.cm = inc.dm + dm
	}
	inc.trend = trend
	inc.lastSum = high + low + cloze
	inc.dm = dm
	inc.Value = volume * (2.*(inc.dm/inc.cm) - 1.) * trend
}

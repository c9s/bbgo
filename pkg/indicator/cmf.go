package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

/*
cmf implements Chaikin Money Flow (CMF) indicator.

Chaikin Money Flow (CMF)
- https://www.fidelity.com/learning-center/trading-investing/technical-analysis/technical-indicator-guide/cmf
- https://www.investopedia.com/terms/c/chaikinmoneyflow.asp

Calculation:
	1. Money Flow Multiplier = ((Close - Low) - (High - Close)) / (High - Low)
	                         = (2 * Close - High - Low) / (High - Low)
	   If High == Low, Multiplier = 0.
	2. Money Flow Volume = Money Flow Multiplier * Volume
	3. CMF = Sum(Money Flow Volume, Window) / Sum(Volume, Window)
*/
//go:generate callbackgen -type CMF
type CMF struct {
	types.SeriesBase
	types.IntervalWindow

	Values   floats.Slice
	mfvQueue *types.Queue
	volQueue *types.Queue

	EndTime time.Time

	UpdateCallbacks []func(value float64)
}

func (inc *CMF) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *CMF) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *CMF) Length() int {
	return len(inc.Values)
}

var _ types.SeriesExtend = &CMF{}

func (inc *CMF) Update(high, low, closePrice, volume float64) {
	if inc.mfvQueue == nil {
		if inc.Window == 0 {
			inc.Window = 20
		}
		inc.mfvQueue = types.NewQueue(inc.Window)
		inc.volQueue = types.NewQueue(inc.Window)
		inc.SeriesBase.Series = inc
	}

	var mfv float64
	if high > low {
		mfm := (2*closePrice - high - low) / (high - low)
		mfv = mfm * volume
	} else {
		mfv = 0
	}

	inc.mfvQueue.Update(mfv)
	inc.volQueue.Update(volume)

	if inc.mfvQueue.Length() < inc.Window {
		return
	}

	var sumMFV float64
	var sumVol float64
	for i := 0; i < inc.Window; i++ {
		sumMFV += inc.mfvQueue.Last(i)
		sumVol += inc.volQueue.Last(i)
	}

	var cmf float64
	if sumVol != 0 {
		cmf = sumMFV / sumVol
	} else {
		cmf = 0
	}

	inc.Values.Push(cmf)
}

func (inc *CMF) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(k.High.Float64(), k.Low.Float64(), k.Close.Float64(), k.Volume.Float64())
	inc.EndTime = k.EndTime.Time()
	if len(inc.Values) > 0 {
		inc.EmitUpdate(inc.Last(0))
	}
}

func (inc *CMF) CalculateAndUpdate(kLines []types.KLine) {
	if len(kLines) < inc.Window {
		return
	}

	var hasNew bool
	for _, k := range kLines {
		if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
			continue
		}

		inc.Update(k.High.Float64(), k.Low.Float64(), k.Close.Float64(), k.Volume.Float64())
		hasNew = true
	}

	if hasNew && len(inc.Values) > 0 {
		inc.EmitUpdate(inc.Last(0))
	}
	inc.EndTime = kLines[len(kLines)-1].EndTime.Time()
}

func (inc *CMF) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *CMF) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func (inc *CMF) BindK(target KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}

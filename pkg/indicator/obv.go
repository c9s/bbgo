package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

/*
obv implements on-balance volume indicator

On-Balance Volume (OBV) Definition
- https://www.investopedia.com/terms/o/onbalancevolume.asp

On-Balance Volume (OBV) is a technical analysis indicator that uses volume information to predict changes in stock price.
The idea behind OBV is that volume precedes price: when the OBV is rising, it means that buyers are becoming more aggressive and
that the stock price is likely to follow suit. When the OBV is falling, it indicates that sellers are becoming more aggressive and
that the stock price is likely to decrease. OBV is calculated by adding the volume on days when the stock price closes higher and
subtracting the volume on days when the stock price closes lower. This running total forms the OBV line, which can then be used
to make predictions about future stock price movements.
*/
//go:generate callbackgen -type OBV
type OBV struct {
	types.SeriesBase
	types.IntervalWindow
	Values   floats.Slice
	PrePrice float64
	EndTime  time.Time

	updateCallbacks []func(value float64)
}

func (inc *OBV) Update(price, volume float64) {
	if len(inc.Values) == 0 {
		inc.SeriesBase.Series = inc
		inc.PrePrice = price
		inc.Values.Push(volume)
		return
	}

	if volume < inc.PrePrice {
		inc.Values.Push(inc.Last() - volume)
	} else {
		inc.Values.Push(inc.Last() + volume)
	}
}

func (inc *OBV) Last() float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *OBV) Index(i int) float64 {
	if len(inc.Values)-i <= 0 {
		return 0.0
	}
	return inc.Values[len(inc.Values)-i-1]
}

var _ types.SeriesExtend = &OBV{}

func (inc *OBV) PushK(k types.KLine) {
	inc.Update(k.Close.Float64(), k.Volume.Float64())
}

func (inc *OBV) CalculateAndUpdate(kLines []types.KLine) {
	for _, k := range kLines {
		if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
			continue
		}

		inc.PushK(k)
	}

	inc.EmitUpdate(inc.Last())
	inc.EndTime = kLines[len(kLines)-1].EndTime.Time()
}

func (inc *OBV) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *OBV) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

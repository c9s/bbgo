package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

/*
obv implements on-balance volume indicator

On-Balance Volume (OBV) Definition
- https://www.investopedia.com/terms/o/onbalancevolume.asp
*/
//go:generate callbackgen -type OBV
type OBV struct {
	types.IntervalWindow
	Values   types.Float64Slice
	PrePrice float64

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

func (inc *OBV) update(kLine types.KLine, priceF KLinePriceMapper) {
	price := priceF(kLine)
	volume := kLine.Volume

	if len(inc.Values) == 0 {
		inc.PrePrice = price
		inc.Values.Push(volume)
		return
	}

	var sign float64 = 0.0
	if volume > inc.PrePrice {
		sign = 1.0
	} else if volume < inc.PrePrice {
		sign = -1.0
	}
	obv := inc.Last() + sign*volume
	inc.Values.Push(obv)
}

func (inc *OBV) Last() float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *OBV) calculateAndUpdate(kLines []types.KLine) {
	var priceF = KLineClosePriceMapper

	for i, k := range kLines {
		if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
			continue
		}

		inc.update(k, priceF)
		inc.EmitUpdate(inc.Last())
		inc.EndTime = kLines[i].EndTime
	}

}
func (inc *OBV) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *OBV) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

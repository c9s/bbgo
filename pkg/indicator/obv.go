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

func (inc *OBV) Update(kLine types.KLine, priceF KLinePriceMapper) {
	price := priceF(kLine)
	volume := kLine.Volume.Float64()

	if len(inc.Values) == 0 {
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

func (inc *OBV) calculateAndUpdate(kLines []types.KLine) {
	var priceF = KLineClosePriceMapper

	for _, k := range kLines {
		if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
			continue
		}
		inc.Update(k, priceF)
	}
	inc.EmitUpdate(inc.Last())
	inc.EndTime = kLines[len(kLines)-1].EndTime.Time()
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

package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

/*
ad implements accumulation/distribution indicator

Accumulation/Distribution Indicator (A/D)
- https://www.investopedia.com/terms/a/accumulationdistribution.asp
*/
//go:generate callbackgen -type AD
type AD struct {
	types.IntervalWindow
	Values   types.Float64Slice
	PrePrice float64

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

func (inc *AD) update(kLine types.KLine) {
	close := kLine.Close.Float64()
	high := kLine.High.Float64()
	low := kLine.Low.Float64()
	volume := kLine.Volume.Float64()

	moneyFlowVolume := ((2*close - high - low) / (high - low)) * volume

	ad := inc.Last() + moneyFlowVolume
	inc.Values.Push(ad)
}

func (inc *AD) Last() float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *AD) calculateAndUpdate(kLines []types.KLine) {
	for i, k := range kLines {
		if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
			continue
		}

		inc.update(k)
		inc.EmitUpdate(inc.Last())
		inc.EndTime = kLines[i].EndTime.Time()
	}

}
func (inc *AD) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *AD) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

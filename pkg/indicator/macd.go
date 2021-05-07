package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type MACD
type MACD struct {
	types.IntervalWindow
	Values      Float64Slice
	ShortPeriod int
	LongPeriod  int
	Smoothing   int

	EndTime time.Time

	UpdateCallbacks []func(value float64)
}

func (inc *MACD) Calculate(kLines []types.KLine) float64 {
	return 0.0
}

func (inc *MACD) calculateAndUpdate(kLines []types.KLine) {

}

func (inc *MACD) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *MACD) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

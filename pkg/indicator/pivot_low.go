package indicator

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type PivotLow
type PivotLow struct {
	types.IntervalWindow

	Values  types.Float64Slice
	EndTime time.Time

	updateCallbacks []func(value float64)
}

func (inc *PivotLow) Last() float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *PivotLow) CalculateAndUpdate(klines []types.KLine) {
	if len(klines) < inc.Window {
		return
	}

	var end = len(klines) - 1
	var lastKLine = klines[end]

	// skip old data
	if inc.EndTime != zeroTime && lastKLine.GetEndTime().Before(inc.EndTime) {
		return
	}

	recentT := klines[end-(inc.Window-1) : end+1]

	l, err := calculatePivotLow(recentT, inc.Window, KLineLowPriceMapper)
	if err != nil {
		log.WithError(err).Error("can not calculate pivots")
		return
	}

	if l > 0.0 {
		inc.Values.Push(l)
	}

	inc.EndTime = klines[end].GetEndTime().Time()
	inc.EmitUpdate(l)
}

func (inc *PivotLow) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *PivotLow) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func calculatePivotLow(klines []types.KLine, window int, valLow KLineValueMapper) (float64, error) {
	length := len(klines)
	if length == 0 || length < window {
		return 0., fmt.Errorf("insufficient elements for calculating with window = %d", window)
	}

	var lows types.Float64Slice
	for _, k := range klines {
		lows.Push(valLow(k))
	}

	pl := 0.
	if lows.Min() == lows.Index(int(window/2.)-1) {
		pl = lows.Min()
	}

	return pl, nil
}

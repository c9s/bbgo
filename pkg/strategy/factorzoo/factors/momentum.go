package factorzoo

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"

	log "github.com/sirupsen/logrus"
)

//go:generate callbackgen -type MOM
type MOM struct {
	types.SeriesBase
	types.IntervalWindow

	// Values
	Values    types.Float64Slice
	LastValue float64

	EndTime time.Time

	UpdateCallbacks []func(val float64)
}

func (inc *MOM) Index(i int) float64 {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Index(i)
}

func (inc *MOM) Last() float64 {
	if inc.Values.Length() == 0 {
		return 0
	}
	return inc.Values.Last()
}

func (inc *MOM) Length() int {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Length()
}

//var _ types.SeriesExtend = &MOM{}

func (inc *MOM) Update(klines []types.KLine) {
	if inc.Values == nil {
		inc.SeriesBase.Series = inc
	}

	if len(klines) < inc.Window {
		return
	}

	var end = len(klines) - 1
	var lastKLine = klines[end]

	if inc.EndTime != zeroTime && lastKLine.GetEndTime().Before(inc.EndTime) {
		return
	}

	var recentT = klines[end-(inc.Window-1) : end+1]

	val, err := calculateMomentum(recentT, inc.Window, indicator.KLineOpenPriceMapper, indicator.KLineClosePriceMapper)
	if err != nil {
		log.WithError(err).Error("can not calculate")
		return
	}
	inc.Values.Push(val)
	inc.LastValue = val

	if len(inc.Values) > indicator.MaxNumOfVOL {
		inc.Values = inc.Values[indicator.MaxNumOfVOLTruncateSize-1:]
	}

	inc.EndTime = klines[end].GetEndTime().Time()

	inc.EmitUpdate(val)

}

func (inc *MOM) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.Update(window)
}

func (inc *MOM) Bind(updater indicator.KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func calculateMomentum(klines []types.KLine, window int, valA KLineValueMapper, valB KLineValueMapper) (float64, error) {
	length := len(klines)
	if length == 0 || length < window {
		return 0.0, fmt.Errorf("insufficient elements for calculating VOL with window = %d", window)
	}

	momentum := (1 - valA(klines[length-1])/valB(klines[length-1])) * -1

	return momentum, nil
}

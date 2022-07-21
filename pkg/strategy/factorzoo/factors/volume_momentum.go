package factorzoo

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"

	log "github.com/sirupsen/logrus"
)

//go:generate callbackgen -type VMOM
type VMOM struct {
	types.SeriesBase
	types.IntervalWindow

	// Values
	Values    types.Float64Slice
	LastValue float64

	EndTime time.Time

	UpdateCallbacks []func(val float64)
}

func (inc *VMOM) Index(i int) float64 {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Index(i)
}

func (inc *VMOM) Last() float64 {
	if inc.Values.Length() == 0 {
		return 0
	}
	return inc.Values.Last()
}

func (inc *VMOM) Length() int {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Length()
}

//var _ types.SeriesExtend = &VMOM{}

func (inc *VMOM) Update(klines []types.KLine) {
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

	val, err := calculateVolumeMomentum(recentT, inc.Window, indicator.KLineVolumeMapper, indicator.KLineClosePriceMapper)
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

func (inc *VMOM) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.Update(window)
}

func (inc *VMOM) Bind(updater indicator.KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func calculateVolumeMomentum(klines []types.KLine, window int, valV KLineValueMapper, valP KLineValueMapper) (float64, error) {
	length := len(klines)
	if length == 0 || length < window {
		return 0.0, fmt.Errorf("insufficient elements for calculating VOL with window = %d", window)
	}

	vma := 0.
	for _, p := range klines[length-window : length-1] {
		vma += valV(p)
	}
	vma /= float64(window)
	momentum := valV(klines[length-1]) / vma //* (valP(klines[length-1-2]) / valP(klines[length-1]))

	return momentum, nil
}

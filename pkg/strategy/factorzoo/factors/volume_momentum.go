package factorzoo

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

// quarterly volume momentum
// assume that the quotient of volume SMA over latest volume will dynamically revert into one.
// so this fraction value is our alpha, PMR

//go:generate callbackgen -type VMOM
type VMOM struct {
	types.SeriesBase
	types.IntervalWindow

	// Values
	Values    floats.Slice
	LastValue float64

	volumes *types.Queue

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

var _ types.SeriesExtend = &VMOM{}

func (inc *VMOM) Update(volume float64) {
	if inc.SeriesBase.Series == nil {
		inc.SeriesBase.Series = inc
		inc.volumes = types.NewQueue(inc.Window)
	}
	inc.volumes.Update(volume)
	if inc.volumes.Length() >= inc.Window {
		v := inc.volumes.Last() / inc.volumes.Mean()
		inc.Values.Push(v)
	}
}

func (inc *VMOM) CalculateAndUpdate(allKLines []types.KLine) {
	if len(inc.Values) == 0 {
		for _, k := range allKLines {
			inc.PushK(k)
		}
		inc.EmitUpdate(inc.Last())
	} else {
		k := allKLines[len(allKLines)-1]
		inc.PushK(k)
		inc.EmitUpdate(inc.Last())
	}
}

func (inc *VMOM) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *VMOM) Bind(updater indicator.KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func (inc *VMOM) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(k.Volume.Float64())
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last())
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

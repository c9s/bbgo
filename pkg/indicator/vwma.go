package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

/*
vwma implements the volume weighted moving average (VWMA) indicator:

Calculation:
	pv = element-wise multiplication of close prices and volumes
	VWMA = SMA(pv, window) / SMA(volumes, window)

Volume Weighted Moving Average
- https://www.motivewave.com/studies/volume_weighted_moving_average.htm
*/
//go:generate callbackgen -type VWMA
type VWMA struct {
	types.SeriesBase
	types.IntervalWindow

	Values         types.Float64Slice
	PriceVolumeSMA *SMA
	VolumeSMA      *SMA

	EndTime time.Time

	updateCallbacks []func(value float64)
}

func (inc *VWMA) Last() float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *VWMA) Index(i int) float64 {
	length := len(inc.Values)
	if length == 0 || length-i-1 < 0 {
		return 0
	}
	return inc.Values[length-i-1]
}

func (inc *VWMA) Length() int {
	return len(inc.Values)
}

var _ types.SeriesExtend = &VWMA{}

func (inc *VWMA) Update(price, volume float64) {
	if inc.PriceVolumeSMA == nil {
		inc.PriceVolumeSMA = &SMA{IntervalWindow: inc.IntervalWindow}
		inc.SeriesBase.Series = inc
	}

	if inc.VolumeSMA == nil {
		inc.VolumeSMA = &SMA{IntervalWindow: inc.IntervalWindow}
	}

	inc.PriceVolumeSMA.Update(price * volume)
	inc.VolumeSMA.Update(volume)

	pv := inc.PriceVolumeSMA.Last()
	v := inc.VolumeSMA.Last()
	vwma := pv / v
	inc.Values.Push(vwma)
}

func (inc *VWMA) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(k.Close.Float64(), k.Volume.Float64())
}


func (inc *VWMA) CalculateAndUpdate(allKLines []types.KLine) {
	if len(allKLines) < inc.Window {
		return
	}

	var last = allKLines[len(allKLines)-1]

	if inc.VolumeSMA == nil {
		for _, k := range allKLines {
			if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
				return
			}

			inc.Update(k.Close.Float64(), k.Volume.Float64())
		}
	} else {
		inc.Update(last.Close.Float64(), last.Volume.Float64())
	}

	inc.EndTime = last.EndTime.Time()
	inc.EmitUpdate(inc.Values.Last())
}

func (inc *VWMA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *VWMA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

package indicator

import (
	"time"

	log "github.com/sirupsen/logrus"

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
	types.IntervalWindow
	Values  types.Float64Slice
	EndTime time.Time

	UpdateCallbacks []func(value float64)
}

func (inc *VWMA) Last() float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}
	return inc.Values[len(inc.Values)-1]
}

func KLinePriceVolumeMapper(k types.KLine) float64 {
	return k.Close.Mul(k.Volume).Float64()
}

func KLineVolumeMapper(k types.KLine) float64 {
	return k.Volume.Float64()
}

func (inc *VWMA) calculateAndUpdate(kLines []types.KLine) {
	if len(kLines) < inc.Window {
		return
	}

	var index = len(kLines) - 1
	var kline = kLines[index]

	if inc.EndTime != zeroTime && kline.EndTime.Before(inc.EndTime) {
		return
	}

	var recentK = kLines[index-(inc.Window-1) : index+1]

	pv, err := calculateSMA(recentK, inc.Window, KLinePriceVolumeMapper)
	if err != nil {
		log.WithError(err).Error("price x volume SMA error")
		return
	}
	v, err := calculateSMA(recentK, inc.Window, KLineVolumeMapper)
	if err != nil {
		log.WithError(err).Error("volume SMA error")
		return
	}

	vwma := pv / v
	inc.Values.Push(vwma)

	if len(inc.Values) > MaxNumOfSMA {
		inc.Values = inc.Values[MaxNumOfSMATruncateSize-1:]
	}

	inc.EndTime = kLines[index].EndTime.Time()

	inc.EmitUpdate(vwma)
}

func (inc *VWMA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *VWMA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

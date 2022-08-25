package factorzoo

import (
	"time"

	"gonum.org/v1/gonum/stat"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

// price volume divergence
// if the correlation of two time series gets smaller, they are diverging.
// so the negative value of the correlation of close price and volume is our alpha, PVD

var zeroTime time.Time

type KLineValueMapper func(k types.KLine) float64

//go:generate callbackgen -type PVD
type PVD struct {
	types.IntervalWindow
	types.SeriesBase

	Values floats.Slice
	Prices *types.Queue
	Volumes *types.Queue
	EndTime time.Time

	updateCallbacks []func(value float64)
}

var _ types.SeriesExtend = &PVD{}

func (inc *PVD) Update(price float64, volume float64) {
	if inc.SeriesBase.Series == nil {
		inc.SeriesBase.Series = inc
		inc.Prices = types.NewQueue(inc.Window)
		inc.Volumes = types.NewQueue(inc.Window)
	}
	inc.Prices.Update(price)
	inc.Volumes.Update(volume)
	if inc.Prices.Length() >= inc.Window && inc.Volumes.Length() >= inc.Window {
		divergence := -types.Correlation(inc.Prices, inc.Volumes, inc.Window)
		inc.Values.Push(divergence)
	}
}

func (inc *PVD) Last() float64 {
	if len(inc.Values) == 0 {
		return 0
	}

	return inc.Values[len(inc.Values)-1]
}

func (inc *PVD) Index(i int) float64 {
	if i >= len(inc.Values) {
		return 0
	}

	return inc.Values[len(inc.Values)-1-i]
}

func (inc *PVD) Length() int {
	return len(inc.Values)
}

func (inc *PVD) CalculateAndUpdate(allKLines []types.KLine) {
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

func (inc *PVD) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *PVD) Bind(updater indicator.KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func (inc *PVD) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(indicator.KLineClosePriceMapper(k), indicator.KLineVolumeMapper(k))
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last())
}

func CalculateKLinesPVD(allKLines []types.KLine, window int) float64 {
	return pvd(indicator.MapKLinePrice(allKLines, indicator.KLineClosePriceMapper), indicator.MapKLinePrice(allKLines, indicator.KLineVolumeMapper), window)
}

func pvd(prices []float64, volumes []float64, window int) float64 {
	var end = len(prices) - 1
	if end == 0 {
		return prices[0]
	}

	divergence := -stat.Correlation(prices[end-window:end], volumes[end-window:end], nil)
	return divergence
}

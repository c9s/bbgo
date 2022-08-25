package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

/*
boll implements the bollinger indicator:

The Basics of Bollinger Bands
- https://www.investopedia.com/articles/technical/102201.asp

Bollinger Bands
- https://www.investopedia.com/terms/b/bollingerbands.asp

Bollinger Bands Technical indicator guide:
- https://www.fidelity.com/learning-center/trading-investing/technical-analysis/technical-indicator-guide/bollinger-bands
*/

//go:generate callbackgen -type BOLL
type BOLL struct {
	types.IntervalWindow

	// K is the multiplier of Std, generally it's 2
	K float64

	SMA    *SMA
	StdDev *StdDev

	UpBand   floats.Slice
	DownBand floats.Slice

	EndTime time.Time

	updateCallbacks []func(sma, upBand, downBand float64)
}

type BandType int

func (inc *BOLL) GetUpBand() types.SeriesExtend {
	return types.NewSeries(&inc.UpBand)
}

func (inc *BOLL) GetDownBand() types.SeriesExtend {
	return types.NewSeries(&inc.DownBand)
}

func (inc *BOLL) GetSMA() types.SeriesExtend {
	return types.NewSeries(inc.SMA)
}

func (inc *BOLL) GetStdDev() types.SeriesExtend {
	return inc.StdDev
}

func (inc *BOLL) LastUpBand() float64 {
	if len(inc.UpBand) == 0 {
		return 0.0
	}

	return inc.UpBand[len(inc.UpBand)-1]
}

func (inc *BOLL) LastDownBand() float64 {
	if len(inc.DownBand) == 0 {
		return 0.0
	}

	return inc.DownBand[len(inc.DownBand)-1]
}

func (inc *BOLL) Update(value float64) {
	if inc.SMA == nil {
		inc.SMA = &SMA{IntervalWindow: inc.IntervalWindow}
	}

	if inc.StdDev == nil {
		inc.StdDev = &StdDev{IntervalWindow: inc.IntervalWindow}
	}

	inc.SMA.Update(value)
	inc.StdDev.Update(value)

	var sma = inc.SMA.Last()
	var stdDev = inc.StdDev.Last()
	var band = inc.K * stdDev

	var upBand = sma + band
	var downBand = sma - band

	inc.UpBand.Push(upBand)
	inc.DownBand.Push(downBand)
}

func (inc *BOLL) BindK(target KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}

func (inc *BOLL) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}
	inc.Update(k.Close.Float64())
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.SMA.Last(), inc.UpBand.Last(), inc.DownBand.Last())
}

func (inc *BOLL) LoadK(allKLines []types.KLine) {
	for _, k := range allKLines {
		inc.PushK(k)
	}

	inc.EmitUpdate(inc.SMA.Last(), inc.UpBand.Last(), inc.DownBand.Last())
}

func (inc *BOLL) CalculateAndUpdate(allKLines []types.KLine) {
	if inc.SMA == nil {
		inc.LoadK(allKLines)
		return
	}

	var last = allKLines[len(allKLines)-1]
	inc.PushK(last)
}

func (inc *BOLL) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	if inc.EndTime != zeroTime && inc.EndTime.Before(inc.EndTime) {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *BOLL) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

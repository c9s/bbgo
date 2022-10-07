package supertrend

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

// LinReg is Linear Regression baseline
type LinReg struct {
	types.SeriesBase
	types.IntervalWindow
	// Values are the slopes of linear regression baseline
	Values  floats.Slice
	klines  types.KLineWindow
	EndTime time.Time
}

// Last slope of linear regression baseline
func (lr *LinReg) Last() float64 {
	if lr.Values.Length() == 0 {
		return 0.0
	}
	return lr.Values.Last()
}

// Index returns the slope of specified index
func (lr *LinReg) Index(i int) float64 {
	if i >= lr.Values.Length() {
		return 0.0
	}

	return lr.Values.Index(i)
}

// Length of the slope values
func (lr *LinReg) Length() int {
	return lr.Values.Length()
}

var _ types.SeriesExtend = &LinReg{}

// Update Linear Regression baseline slope
func (lr *LinReg) Update(kline types.KLine) {
	lr.klines.Add(kline)
	lr.klines.Truncate(lr.Window)
	if len(lr.klines) < lr.Window {
		lr.Values.Push(0)
		return
	}

	var sumX, sumY, sumXSqr, sumXY float64 = 0, 0, 0, 0
	end := len(lr.klines) - 1 // The last kline
	for i := end; i >= end-lr.Window+1; i-- {
		val := lr.klines[i].GetClose().Float64()
		per := float64(end - i + 1)
		sumX += per
		sumY += val
		sumXSqr += per * per
		sumXY += val * per
	}
	length := float64(lr.Window)
	slope := (length*sumXY - sumX*sumY) / (length*sumXSqr - sumX*sumX)
	average := sumY / length
	endPrice := average - slope*sumX/length + slope
	startPrice := endPrice + slope*(length-1)
	lr.Values.Push((endPrice - startPrice) / (length - 1))

	log.Debugf("linear regression baseline slope: %f", lr.Last())
}

func (lr *LinReg) BindK(target indicator.KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, lr.PushK))
}

func (lr *LinReg) PushK(k types.KLine) {
	var zeroTime = time.Time{}
	if lr.EndTime != zeroTime && k.EndTime.Before(lr.EndTime) {
		return
	}

	lr.Update(k)
	lr.EndTime = k.EndTime.Time()
}

func (lr *LinReg) LoadK(allKLines []types.KLine) {
	for _, k := range allKLines {
		lr.PushK(k)
	}
}

// GetSignal get linear regression signal
func (lr *LinReg) GetSignal() types.Direction {
	var lrSignal types.Direction = types.DirectionNone

	switch {
	case lr.Last() > 0:
		lrSignal = types.DirectionUp
	case lr.Last() < 0:
		lrSignal = types.DirectionDown
	}

	return lrSignal
}

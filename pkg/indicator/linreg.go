package indicator

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

var logLinReg = logrus.WithField("indicator", "LinReg")

// LinReg is Linear Regression baseline
//
//go:generate callbackgen -type LinReg
type LinReg struct {
	types.SeriesBase
	types.IntervalWindow

	// Values are the slopes of linear regression baseline
	Values floats.Slice
	// ValueRatios are the ratio of slope to the price
	ValueRatios floats.Slice

	klines types.KLineWindow

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

// Last slope of linear regression baseline
func (lr *LinReg) Last(i int) float64 {
	return lr.Values.Last(i)
}

// LastRatio of slope to price
func (lr *LinReg) LastRatio() float64 {
	if lr.ValueRatios.Length() == 0 {
		return 0.0
	}
	return lr.ValueRatios.Last(0)
}

// Index returns the slope of specified index
func (lr *LinReg) Index(i int) float64 {
	return lr.Values.Last(i)
}

// IndexRatio returns the slope ratio
func (lr *LinReg) IndexRatio(i int) float64 {
	if i >= lr.ValueRatios.Length() {
		return 0.0
	}

	return lr.ValueRatios.Last(i)
}

// Length of the slope values
func (lr *LinReg) Length() int {
	return lr.Values.Length()
}

// LengthRatio of the slope ratio values
func (lr *LinReg) LengthRatio() int {
	return lr.ValueRatios.Length()
}

var _ types.SeriesExtend = &LinReg{}

// Update Linear Regression baseline slope
func (lr *LinReg) Update(kline types.KLine) {
	lr.klines.Add(kline)
	lr.klines.Truncate(lr.Window)
	if len(lr.klines) < lr.Window {
		lr.Values.Push(0)
		lr.ValueRatios.Push(0)
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
	lr.ValueRatios.Push(lr.Values.Last(0) / kline.GetClose().Float64())

	logLinReg.Debugf("linear regression baseline slope: %f", lr.Last(0))
}

func (lr *LinReg) BindK(target KLineClosedEmitter, symbol string, interval types.Interval) {
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

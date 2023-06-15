package factorzoo

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

// gap jump momentum
// if the gap between current open price and previous close price gets larger
// meaning an opening price jump was happened, the larger momentum we get is our alpha, MOM

//go:generate callbackgen -type MOM
type MOM struct {
	types.SeriesBase
	types.IntervalWindow

	// Values
	Values    floats.Slice
	LastValue float64

	opens  *types.Queue
	closes *types.Queue

	EndTime time.Time

	UpdateCallbacks []func(val float64)
}

func (inc *MOM) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *MOM) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *MOM) Length() int {
	return inc.Values.Length()
}

// var _ types.SeriesExtend = &MOM{}

func (inc *MOM) Update(open, close float64) {
	if inc.SeriesBase.Series == nil {
		inc.SeriesBase.Series = inc
		inc.opens = types.NewQueue(inc.Window)
		inc.closes = types.NewQueue(inc.Window + 1)
	}
	inc.opens.Update(open)
	inc.closes.Update(close)
	if inc.opens.Length() >= inc.Window && inc.closes.Length() >= inc.Window {
		gap := inc.opens.Last(0)/inc.closes.Index(1) - 1
		inc.Values.Push(gap)
	}
}

func (inc *MOM) CalculateAndUpdate(allKLines []types.KLine) {
	if len(inc.Values) == 0 {
		for _, k := range allKLines {
			inc.PushK(k)
		}
		inc.EmitUpdate(inc.Last(0))
	} else {
		k := allKLines[len(allKLines)-1]
		inc.PushK(k)
		inc.EmitUpdate(inc.Last(0))
	}
}

func (inc *MOM) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *MOM) Bind(updater indicator.KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func (inc *MOM) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(k.Open.Float64(), k.Close.Float64())
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last(0))
}

func calculateMomentum(klines []types.KLine, window int, valA KLineValueMapper, valB KLineValueMapper) (float64, error) {
	length := len(klines)
	if length == 0 || length < window {
		return 0.0, fmt.Errorf("insufficient elements for calculating VOL with window = %d", window)
	}

	momentum := (1 - valA(klines[length-1])/valB(klines[length-1])) * -1

	return momentum, nil
}

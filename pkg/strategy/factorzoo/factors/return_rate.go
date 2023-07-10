package factorzoo

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

// simply internal return rate over certain timeframe(interval)

//go:generate callbackgen -type RR
type RR struct {
	types.IntervalWindow
	types.SeriesBase

	prices  *types.Queue
	Values  floats.Slice
	EndTime time.Time

	updateCallbacks []func(value float64)
}

var _ types.SeriesExtend = &RR{}

func (inc *RR) Update(price float64) {
	if inc.SeriesBase.Series == nil {
		inc.SeriesBase.Series = inc
		inc.prices = types.NewQueue(inc.Window)
	}
	inc.prices.Update(price)
	irr := inc.prices.Last(0)/inc.prices.Index(1) - 1
	inc.Values.Push(irr)

}

func (inc *RR) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *RR) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *RR) Length() int {
	return len(inc.Values)
}

func (inc *RR) CalculateAndUpdate(allKLines []types.KLine) {
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

func (inc *RR) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *RR) Bind(updater indicator.KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func (inc *RR) BindK(target indicator.KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}

func (inc *RR) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(types.KLineClosePriceMapper(k))
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last(0))
}

func (inc *RR) LoadK(allKLines []types.KLine) {
	for _, k := range allKLines {
		inc.PushK(k)
	}
	inc.EmitUpdate(inc.Last(0))
}

// func calculateReturn(klines []types.KLine, window int, val KLineValueMapper) (float64, error) {
//	length := len(klines)
//	if length == 0 || length < window {
//		return 0.0, fmt.Errorf("insufficient elements for calculating VOL with window = %d", window)
//	}
//
//	rate := val(klines[length-1])/val(klines[length-2]) - 1
//
//	return rate, nil
// }

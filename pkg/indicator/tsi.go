package indicator

import (
	"math"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: True Strength Index
// Refer URL: https://www.investopedia.com/terms/t/tsi.asp
//go:generate callbackgen -type TSI
type TSI struct {
	types.SeriesBase
	types.IntervalWindow
	PrevValue       float64
	Values          floats.Slice
	Pcs             *EWMA
	Pcds            *EWMA
	Apcs            *EWMA
	Apcds           *EWMA
	updateCallbacks []func(value float64)
}

func (inc *TSI) Update(value float64) {
	if inc.Pcs == nil {
		inc.Pcs = &EWMA{
			IntervalWindow: types.IntervalWindow{
				Window:   25,
				Interval: inc.Interval,
			},
		}
		inc.Pcds = &EWMA{
			IntervalWindow: types.IntervalWindow{
				Window:   13,
				Interval: inc.Interval,
			},
		}
		inc.Apcs = &EWMA{
			IntervalWindow: types.IntervalWindow{
				Window:   25,
				Interval: inc.Interval,
			},
		}
		inc.Apcds = &EWMA{
			IntervalWindow: types.IntervalWindow{
				Window:   13,
				Interval: inc.Interval,
			},
		}
		inc.SeriesBase.Series = inc
	}
	if inc.PrevValue == 0 {
		inc.PrevValue = value
		return
	}
	pc := value - inc.PrevValue
	inc.Pcs.Update(pc)
	inc.Pcds.Update(inc.Pcs.Last())
	apc := math.Abs(pc)
	inc.Apcs.Update(apc)
	inc.Apcds.Update(inc.Apcs.Last())
	tsi := (inc.Pcds.Last() / inc.Apcds.Last()) * 100.
	inc.Values.Push(tsi)
	if inc.Values.Length() > MaxNumOfEWMA {
		inc.Values = inc.Values[MaxNumOfEWMATruncateSize-1:]
	}
	inc.PrevValue = value
}

func (inc *TSI) Last() float64 {
	return inc.Values.Last()
}

func (inc *TSI) Index(i int) float64 {
	return inc.Values.Index(i)
}

func (inc *TSI) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
}

var _ types.SeriesExtend = &TSI{}

func (inc *TSI) CalculateAndUpdate(allKLines []types.KLine) {
	if inc.PrevValue == 0 {
		for _, k := range allKLines {
			inc.PushK(k)
			inc.EmitUpdate(inc.Last())
		}
	} else {
		k := allKLines[len(allKLines)-1]
		inc.PushK(k)
		inc.EmitUpdate(inc.Last())
	}
}

func (inc *TSI) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}
	inc.CalculateAndUpdate(window)
}

func (inc *TSI) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

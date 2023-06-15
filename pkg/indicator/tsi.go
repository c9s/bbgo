package indicator

import (
	"math"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: True Strength Index
// Refer URL: https://www.investopedia.com/terms/t/tsi.asp
//
//go:generate callbackgen -type TSI
type TSI struct {
	types.SeriesBase
	types.Interval
	FastWindow      int
	SlowWindow      int
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
		if inc.FastWindow == 0 {
			inc.FastWindow = 13
		}
		if inc.SlowWindow == 0 {
			inc.SlowWindow = 25
		}
		inc.Pcs = &EWMA{
			IntervalWindow: types.IntervalWindow{
				Window:   inc.SlowWindow,
				Interval: inc.Interval,
			},
		}
		inc.Pcds = &EWMA{
			IntervalWindow: types.IntervalWindow{
				Window:   inc.FastWindow,
				Interval: inc.Interval,
			},
		}
		inc.Apcs = &EWMA{
			IntervalWindow: types.IntervalWindow{
				Window:   inc.SlowWindow,
				Interval: inc.Interval,
			},
		}
		inc.Apcds = &EWMA{
			IntervalWindow: types.IntervalWindow{
				Window:   inc.FastWindow,
				Interval: inc.Interval,
			},
		}
		inc.SeriesBase.Series = inc
		inc.PrevValue = value
		return
	}
	pc := value - inc.PrevValue
	inc.PrevValue = value
	inc.Pcs.Update(pc)
	apc := math.Abs(pc)
	inc.Apcs.Update(apc)

	inc.Pcds.Update(inc.Pcs.Last(0))
	inc.Apcds.Update(inc.Apcs.Last(0))

	tsi := (inc.Pcds.Last(0) / inc.Apcds.Last(0)) * 100.
	inc.Values.Push(tsi)
	if inc.Values.Length() > MaxNumOfEWMA {
		inc.Values = inc.Values[MaxNumOfEWMATruncateSize-1:]
	}
}

func (inc *TSI) Length() int {
	return inc.Values.Length()
}

func (inc *TSI) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *TSI) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *TSI) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
}

var _ types.SeriesExtend = &TSI{}

func (inc *TSI) BindK(target KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}

package indicator

import (
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Commodity Channel Index
// Refer URL: https://www.investopedia.com/terms/c/commoditychannelindex.asp
//go:generate callbackgen -type CCI
type CCI struct {
	types.IntervalWindow
	HLC3 types.Float64Slice
	TypicalPrice types.Float64Slice
	MA types.Float64Slice
	Values types.Float64Slice

	UpdateCallbacks []func(value float64)
}

func (inc *CCI) Update(high, low, cloze float64) {
	if len(inc.TypicalPrice) == 0 {
		inc.HLC3.Push((high + low + cloze) / 3.)
		inc.TypicalPrice.Push(inc.HLC3.Last())
		return
	} else if len(inc.TypicalPrice) > MaxNumOfEWMA {
		inc.TypicalPrice = inc.TypicalPrice[MaxNumOfEWMATruncateSize-1:]
		inc.HLC3 = inc.HLC3[MaxNumOfEWMATruncateSize-1:]
	}

	hlc3 := (high + low + cloze) / 3.
	tp := inc.TypicalPrice.Last() - inc.HLC3.Index(inc.Window) + hlc3
	inc.TypicalPrice.Push(tp)
	if len(inc.TypicalPrice) < inc.Window {
		return
	}
	ma := 0.
	for i := 0; i < inc.Window; i++ {
		ma += inc.TypicalPrice.Index(i)
	}
	ma /= float64(inc.Window)
	inc.MA.Push(ma)
	if len(inc.MA) < MaxNumOfEWMA {
		inc.MA = inc.MA[MaxNumOfEWMATruncateSize-1:]
	}
	md := 0.
	for i := 0; i < inc.Window; i++ {
		md += inc.TypicalPrice.Index(i) - inc.MA.Index(i)
	}
	md /= float64(inc.Window)

	cci := (tp - ma) / (0.15 * md)

	inc.Values.Push(cci)
	if len(inc.Values) > MaxNumOfEWMA {
		inc.Values = inc.Values[MaxNumOfEWMATruncateSize-1:]
	}
}

func (inc *CCI) Last() float64 {
	if len(inc.Values) == 0 {
		return 0
	}
	return inc.Values[len(inc.Values) - 1]
}

func (inc *CCI) Index(i int) float64 {
	if i >= len(inc.Values) {
		return 0
	}
	return inc.Values[len(inc.Values)-1-i]
}

func (inc *CCI) Length() int {
	return len(inc.Values)
}

var _ types.Series = &CCI{}

func (inc *CCI) calculateAndUpdate(allKLines []types.KLine) {
	if inc.HLC3.Length() == 0 {
		for _, k := range allKLines {
			inc.Update(k.High.Float64(), k.Low.Float64(), k.Close.Float64())
			inc.EmitUpdate(inc.Last())
		}
	} else {
		k := allKLines[len(allKLines)-1]
		inc.Update(k.High.Float64(), k.Low.Float64(), k.Close.Float64())
		inc.EmitUpdate(inc.Last())
	}
}

func (inc *CCI) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *CCI) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

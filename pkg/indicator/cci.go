package indicator

import (
	"math"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Commodity Channel Index
// Refer URL: http://www.andrewshamlet.net/2017/07/08/python-tutorial-cci
// with modification of ddof=0 to let standard deviation to be divided by N instead of N-1
//
// CCI = (Typical Price  -  n-period SMA of TP) / (Constant x Mean Deviation)
//
// Typical Price (TP) = (High + Low + Close)/3
//
// Constant = .015
//
// The Commodity Channel Index (CCI) is a technical analysis indicator that is used to identify potential overbought or oversold conditions
// in a security's price. It was originally developed for use in commodity markets, but can be applied to any security that has a sufficient
// amount of price data. The CCI is calculated by taking the difference between the security's typical price (the average of its high, low, and
// closing prices) and its moving average, and then dividing the result by the mean absolute deviation of the typical price. This resulting value
// is then plotted as a line on the price chart, with values above +100 indicating overbought conditions and values below -100 indicating
// oversold conditions. The CCI can be used by traders to identify potential entry and exit points for trades, or to confirm other technical
// analysis signals.

//go:generate callbackgen -type CCI
type CCI struct {
	types.SeriesBase
	types.IntervalWindow
	Input        floats.Slice
	TypicalPrice floats.Slice
	MA           floats.Slice
	Values       floats.Slice

	UpdateCallbacks []func(value float64)
}

func (inc *CCI) Update(value float64) {
	if len(inc.TypicalPrice) == 0 {
		inc.SeriesBase.Series = inc
		inc.TypicalPrice.Push(value)
		inc.Input.Push(value)
		return
	} else if len(inc.TypicalPrice) > MaxNumOfEWMA {
		inc.TypicalPrice = inc.TypicalPrice[MaxNumOfEWMATruncateSize-1:]
		inc.Input = inc.Input[MaxNumOfEWMATruncateSize-1:]
	}

	inc.Input.Push(value)
	tp := inc.TypicalPrice.Last(0) - inc.Input.Last(inc.Window) + value
	inc.TypicalPrice.Push(tp)
	if len(inc.Input) < inc.Window {
		return
	}

	ma := tp / float64(inc.Window)
	inc.MA.Push(ma)
	if len(inc.MA) > MaxNumOfEWMA {
		inc.MA = inc.MA[MaxNumOfEWMATruncateSize-1:]
	}

	md := 0.
	for i := 0; i < inc.Window; i++ {
		diff := inc.Input.Last(i) - ma
		md += diff * diff
	}
	md = math.Sqrt(md / float64(inc.Window))

	cci := (value - ma) / (0.015 * md)

	inc.Values.Push(cci)
	if len(inc.Values) > MaxNumOfEWMA {
		inc.Values = inc.Values[MaxNumOfEWMATruncateSize-1:]
	}
}

func (inc *CCI) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *CCI) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *CCI) Length() int {
	return len(inc.Values)
}

var _ types.SeriesExtend = &CCI{}

func (inc *CCI) PushK(k types.KLine) {
	inc.Update(k.High.Add(k.Low).Add(k.Close).Div(three).Float64())
}

func (inc *CCI) CalculateAndUpdate(allKLines []types.KLine) {
	if inc.TypicalPrice.Length() == 0 {
		for _, k := range allKLines {
			inc.PushK(k)
			inc.EmitUpdate(inc.Last(0))
		}
	} else {
		k := allKLines[len(allKLines)-1]
		inc.PushK(k)
		inc.EmitUpdate(inc.Last(0))
	}
}

package indicator

import (
	"math"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Arnaud Legoux Moving Average
// Refer: https://capital.com/arnaud-legoux-moving-average
// Also check https://github.com/DaveSkender/Stock.Indicators/blob/main/src/a-d/Alma/Alma.cs
//
// The Arnaud Legoux Moving Average (ALMA) is a technical analysis indicator that is used to smooth price data and reduce the lag associated
// with traditional moving averages. It was developed by Arnaud Legoux and is based on the weighted moving average, with the weighting factors
// determined using a Gaussian function. The ALMA is calculated by taking the weighted moving average of the input data using weighting factors
// that are based on the standard deviation of the data and the specified length of the moving average. This resulting average is then plotted
// on the price chart as a line, which can be used to make predictions about future price movements. The ALMA is typically more responsive to
// changes in the underlying data than a simple moving average, but may be less reliable in trending markets.
//
// @param offset: Gaussian applied to the combo line. 1->ema, 0->sma
// @param sigma: the standard deviation applied to the combo line. This makes the combo line sharper
//
//go:generate callbackgen -type ALMA
type ALMA struct {
	types.SeriesBase
	types.IntervalWindow         // required
	Offset               float64 // required: recommend to be 0.5
	Sigma                int     // required: recommend to be 5
	weight               []float64
	sum                  float64
	input                []float64
	Values               floats.Slice
	UpdateCallbacks      []func(value float64)
}

const MaxNumOfALMA = 5_000
const MaxNumOfALMATruncateSize = 100

func (inc *ALMA) Update(value float64) {
	if inc.weight == nil {
		inc.SeriesBase.Series = inc
		inc.weight = make([]float64, inc.Window)
		m := inc.Offset * (float64(inc.Window) - 1.)
		s := float64(inc.Window) / float64(inc.Sigma)
		inc.sum = 0.
		for i := 0; i < inc.Window; i++ {
			diff := float64(i) - m
			wt := math.Exp(-diff * diff / 2. / s / s)
			inc.sum += wt
			inc.weight[i] = wt
		}
	}
	inc.input = append(inc.input, value)
	if len(inc.input) >= inc.Window {
		weightedSum := 0.0
		inc.input = inc.input[len(inc.input)-inc.Window:]
		for i := 0; i < inc.Window; i++ {
			weightedSum += inc.weight[inc.Window-i-1] * inc.input[i]
		}
		inc.Values.Push(weightedSum / inc.sum)
		if len(inc.Values) > MaxNumOfALMA {
			inc.Values = inc.Values[MaxNumOfALMATruncateSize-1:]
		}
	}
}

func (inc *ALMA) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *ALMA) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *ALMA) Length() int {
	return len(inc.Values)
}

var _ types.SeriesExtend = &ALMA{}

func (inc *ALMA) CalculateAndUpdate(allKLines []types.KLine) {
	if inc.input == nil {
		for _, k := range allKLines {
			inc.Update(k.Close.Float64())
			inc.EmitUpdate(inc.Last(0))
		}
		return
	}
	inc.Update(allKLines[len(allKLines)-1].Close.Float64())
	inc.EmitUpdate(inc.Last(0))
}

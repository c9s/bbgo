package indicator

import (
	"math"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Arnaud Legoux Moving Average
// Refer: https://capital.com/arnaud-legoux-moving-average
// Also check https://github.com/DaveSkender/Stock.Indicators/blob/main/src/a-d/Alma/Alma.cs
// @param offset: Gaussian applied to the combo line. 1->ema, 0->sma
// @param sigma: the standard deviation applied to the combo line. This makes the combo line sharper
//go:generate callbackgen -type ALMA
type ALMA struct {
	types.SeriesBase
	types.IntervalWindow         // required
	Offset               float64 // required: recommend to be 0.5
	Sigma                int     // required: recommend to be 5
	weight               []float64
	sum                  float64
	input           []float64
	Values          floats.Slice
	UpdateCallbacks []func(value float64)
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

func (inc *ALMA) Last() float64 {
	if len(inc.Values) == 0 {
		return 0
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *ALMA) Index(i int) float64 {
	if i >= len(inc.Values) {
		return 0
	}
	return inc.Values[len(inc.Values)-i-1]
}

func (inc *ALMA) Length() int {
	return len(inc.Values)
}

var _ types.SeriesExtend = &ALMA{}

func (inc *ALMA) CalculateAndUpdate(allKLines []types.KLine) {
	if inc.input == nil {
		for _, k := range allKLines {
			inc.Update(k.Close.Float64())
			inc.EmitUpdate(inc.Last())
		}
		return
	}
	inc.Update(allKLines[len(allKLines)-1].Close.Float64())
	inc.EmitUpdate(inc.Last())
}

func (inc *ALMA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}
	inc.CalculateAndUpdate(window)
}

func (inc *ALMA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

package indicator

import (
	"math"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Variable Index Dynamic Average
// Refer URL: https://metatrader5.com/en/terminal/help/indicators/trend_indicators/vida
// The Variable Index Dynamic Average (VIDYA) is a technical analysis indicator that is used to smooth price data and reduce the lag
// associated with traditional moving averages. It is calculated by taking the weighted moving average of the input data, with the
// weighting factors determined using a variable index that is based on the standard deviation of the data and the specified length of
// the moving average. This resulting average is then plotted on the price chart as a line, which can be used to make predictions about
// future price movements. The VIDYA is typically more responsive to changes in the underlying data than a simple moving average, but may
// be less reliable in trending markets.

//go:generate callbackgen -type VIDYA
type VIDYA struct {
	types.SeriesBase
	types.IntervalWindow
	Values floats.Slice
	input  floats.Slice

	updateCallbacks []func(value float64)
}

func (inc *VIDYA) Update(value float64) {
	if inc.Values.Length() == 0 {
		inc.SeriesBase.Series = inc
		inc.Values.Push(value)
		inc.input.Push(value)
		return
	}
	inc.input.Push(value)
	if len(inc.input) > MaxNumOfEWMA {
		inc.input = inc.input[MaxNumOfEWMATruncateSize-1:]
	}
	/*upsum := 0.
	downsum := 0.
	for i := 0; i < inc.Window; i++ {
		if len(inc.input) <= i+1 {
			break
		}
		diff := inc.input.Index(i) - inc.input.Index(i+1)
		if diff > 0 {
			upsum += diff
		} else {
			downsum += -diff
		}

	}
	if upsum == 0 && downsum == 0 {
		return
	}
	CMO := math.Abs((upsum - downsum) / (upsum + downsum))*/
	change := types.Change(&inc.input)
	CMO := math.Abs(types.Sum(change, inc.Window) / types.Sum(types.Abs(change), inc.Window))
	alpha := 2. / float64(inc.Window+1)
	inc.Values.Push(value*alpha*CMO + inc.Values.Last(0)*(1.-alpha*CMO))
	if inc.Values.Length() > MaxNumOfEWMA {
		inc.Values = inc.Values[MaxNumOfEWMATruncateSize-1:]
	}
}

func (inc *VIDYA) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *VIDYA) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *VIDYA) Length() int {
	return inc.Values.Length()
}

var _ types.SeriesExtend = &VIDYA{}

func (inc *VIDYA) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
}

func (inc *VIDYA) CalculateAndUpdate(allKLines []types.KLine) {
	if inc.input.Length() == 0 {
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

func (inc *VIDYA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *VIDYA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

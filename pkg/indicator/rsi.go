package indicator

import (
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

/*
rsi implements Relative Strength Index (RSI)

https://www.investopedia.com/terms/r/rsi.asp
*/
//go:generate callbackgen -type RSI
type RSI struct {
	types.SeriesBase
	types.IntervalWindow
	Values          floats.Slice
	Prices          floats.Slice
	PreviousAvgLoss float64
	PreviousAvgGain float64

	EndTime         time.Time
	updateCallbacks []func(value float64)
}

func (inc *RSI) Update(price float64) {
	if len(inc.Prices) == 0 {
		inc.SeriesBase.Series = inc
	}
	inc.Prices.Push(price)

	if len(inc.Prices) < inc.Window+1 {
		return
	}

	var avgGain float64
	var avgLoss float64
	if len(inc.Prices) == inc.Window+1 {
		priceDifferences := inc.Prices.Diff()

		avgGain = priceDifferences.PositiveValuesOrZero().Abs().Sum() / float64(inc.Window)
		avgLoss = priceDifferences.NegativeValuesOrZero().Abs().Sum() / float64(inc.Window)
	} else {
		difference := price - inc.Prices[len(inc.Prices)-2]
		currentGain := math.Max(difference, 0)
		currentLoss := -math.Min(difference, 0)

		avgGain = (inc.PreviousAvgGain*13 + currentGain) / float64(inc.Window)
		avgLoss = (inc.PreviousAvgLoss*13 + currentLoss) / float64(inc.Window)
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))
	inc.Values.Push(rsi)

	inc.PreviousAvgGain = avgGain
	inc.PreviousAvgLoss = avgLoss
}

func (inc *RSI) Last() float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *RSI) Index(i int) float64 {
	length := len(inc.Values)
	if length <= 0 || length-i-1 < 0 {
		return 0.0
	}
	return inc.Values[length-i-1]
}

func (inc *RSI) Length() int {
	return len(inc.Values)
}

var _ types.SeriesExtend = &RSI{}

func (inc *RSI) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
}

func (inc *RSI) CalculateAndUpdate(kLines []types.KLine) {
	for _, k := range kLines {
		if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
			continue
		}

		inc.PushK(k)
	}

	inc.EmitUpdate(inc.Last())
	inc.EndTime = kLines[len(kLines)-1].EndTime.Time()
}

func (inc *RSI) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *RSI) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

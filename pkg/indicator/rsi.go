package indicator

import (
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// rsi implements Relative Strength Index (RSI)
// https://www.investopedia.com/terms/r/rsi.asp
//
// The Relative Strength Index (RSI) is a technical analysis indicator that is used to measure the strength of a security's price. It is
// calculated by taking the average of the gains and losses of the security over a specified period of time, and then dividing the average gain
// by the average loss. This resulting value is then plotted as a line on the price chart, with values above 70 indicating overbought conditions
// and values below 30 indicating oversold conditions. The RSI can be used by traders to identify potential entry and exit points for trades,
// or to confirm other technical analysis signals. It is typically used in conjunction with other indicators to provide a more comprehensive
// view of the security's price.

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

func (inc *RSI) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *RSI) Index(i int) float64 {
	return inc.Last(i)
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

	inc.EmitUpdate(inc.Last(0))
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

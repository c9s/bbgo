package indicator

import (
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

/*
rsi implements Relative Strength Index (RSI)

https://www.investopedia.com/terms/r/rsi.asp
*/
//go:generate callbackgen -type RSI
type RSI struct {
	types.IntervalWindow
	Values          types.Float64Slice
	Prices          types.Float64Slice
	PreviousAvgLoss float64
	PreviousAvgGain float64

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

func (inc *RSI) Update(kline types.KLine, priceF KLinePriceMapper) {
	price := priceF(kline)
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

func (inc *RSI) calculateAndUpdate(kLines []types.KLine) {
	var priceF = KLineClosePriceMapper

	for _, k := range kLines {
		if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
			continue
		}
		inc.Update(k, priceF)
	}

	inc.EmitUpdate(inc.Last())
	inc.EndTime = kLines[len(kLines)-1].EndTime.Time()
}

func (inc *RSI) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *RSI) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

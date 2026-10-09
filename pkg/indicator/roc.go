package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

const MaxNumOfROC = 5_000
const MaxNumOfROCTruncateSize = 100

// Refer: Price Rate of Change (ROC)
// Refer URL: https://www.investopedia.com/terms/p/pricerateofchange.asp
// The Price Rate of Change (ROC) is a momentum-based technical indicator that measures the
// percentage change in price between the current price and the price a certain number of periods ago.
// The ROC indicator is calculated by taking the current price and subtracting the price "n" periods ago,
// dividing the difference by the price "n" periods ago, and multiplying the result by 100.

//go:generate callbackgen -type ROC
type ROC struct {
	types.SeriesBase
	types.IntervalWindow

	Prices  floats.Slice
	Values  floats.Slice
	EndTime time.Time

	UpdateCallbacks []func(value float64)
}

func (inc *ROC) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *ROC) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *ROC) Length() int {
	return inc.Values.Length()
}

var _ types.SeriesExtend = &ROC{}

func (inc *ROC) Update(price float64) {
	if len(inc.Prices) == 0 {
		inc.SeriesBase.Series = inc
	}

	inc.Prices.Push(price)
	if len(inc.Prices) <= inc.Window {
		return
	}

	prevPrice := inc.Prices[len(inc.Prices)-1-inc.Window]
	if prevPrice != 0 {
		roc := ((price - prevPrice) / prevPrice) * 100.0
		inc.Values.Push(roc)
	} else {
		inc.Values.Push(0.0)
	}

	if len(inc.Values) > MaxNumOfROC {
		inc.Values = inc.Values[MaxNumOfROCTruncateSize-1:]
	}
	if len(inc.Prices) > MaxNumOfROC+inc.Window {
		inc.Prices = inc.Prices[MaxNumOfROCTruncateSize-1:]
	}
}

func (inc *ROC) BindK(target KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}

func (inc *ROC) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(k.Close.Float64())
	inc.EndTime = k.EndTime.Time()
	if len(inc.Values) > 0 {
		inc.EmitUpdate(inc.Values.Last(0))
	}
}

func (inc *ROC) LoadK(allKLines []types.KLine) {
	for _, k := range allKLines {
		inc.PushK(k)
	}
}

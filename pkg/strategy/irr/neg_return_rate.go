package irr

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

var zeroTime time.Time

// simple negative internal return rate over certain timeframe(interval)

//go:generate callbackgen -type NRR
type NRR struct {
	types.IntervalWindow
	types.SeriesBase

	RankingWindow int

	Prices       *types.Queue
	Values       floats.Slice
	RankedValues floats.Slice

	EndTime time.Time

	updateCallbacks []func(value float64)
}

var _ types.SeriesExtend = &NRR{}

func (inc *NRR) Update(price float64) {
	if inc.SeriesBase.Series == nil {
		inc.SeriesBase.Series = inc
		inc.Prices = types.NewQueue(inc.Window)
	}
	inc.Prices.Update(price)
	if inc.Prices.Length() < inc.Window {
		return
	}
	irr := (inc.Prices.Last() / inc.Prices.Index(inc.Window-1)) - 1

	inc.Values.Push(-irr)                                                                  // neg ret here
	inc.RankedValues.Push(inc.Rank(inc.RankingWindow).Last() / float64(inc.RankingWindow)) // ranked neg ret here

}

func (inc *NRR) Last() float64 {
	if len(inc.Values) == 0 {
		return 0
	}

	return inc.Values[len(inc.Values)-1]
}

func (inc *NRR) Index(i int) float64 {
	if i >= len(inc.Values) {
		return 0
	}

	return inc.Values[len(inc.Values)-1-i]
}

func (inc *NRR) Length() int {
	return len(inc.Values)
}

func (inc *NRR) BindK(target indicator.KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}

func (inc *NRR) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(indicator.KLineClosePriceMapper(k))
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last())
}

func (inc *NRR) LoadK(allKLines []types.KLine) {
	for _, k := range allKLines {
		inc.PushK(k)
	}
	inc.EmitUpdate(inc.Last())
}

//func calculateReturn(klines []types.KLine, window int, val KLineValueMapper) (float64, error) {
//	length := len(klines)
//	if length == 0 || length < window {
//		return 0.0, fmt.Errorf("insufficient elements for calculating VOL with window = %d", window)
//	}
//
//	rate := val(klines[length-1])/val(klines[length-2]) - 1
//
//	return rate, nil
//}

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
	delay         bool
	prices        *types.Queue

	Values       floats.Slice
	RankedValues floats.Slice
	ReturnValues floats.Slice

	EndTime time.Time

	updateCallbacks []func(value float64)
}

var _ types.SeriesExtend = &NRR{}

func (inc *NRR) Update(openPrice, closePrice float64) {
	if inc.SeriesBase.Series == nil {
		inc.SeriesBase.Series = inc
		inc.prices = types.NewQueue(inc.Window)
	}
	inc.prices.Update(closePrice)

	// D0
	nirr := (openPrice - closePrice) / openPrice
	irr := (closePrice - openPrice) / openPrice
	if inc.prices.Length() >= inc.Window && inc.delay {
		// D1
		nirr = -1 * ((inc.prices.Last(0) / inc.prices.Index(inc.Window-1)) - 1)
		irr = (inc.prices.Last(0) / inc.prices.Index(inc.Window-1)) - 1
	}

	inc.Values.Push(nirr)                                                                   // neg ret here
	inc.RankedValues.Push(inc.Rank(inc.RankingWindow).Last(0) / float64(inc.RankingWindow)) // ranked neg ret here
	inc.ReturnValues.Push(irr)
}

func (inc *NRR) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *NRR) Index(i int) float64 {
	return inc.Last(i)
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

	inc.Update(types.KLineOpenPriceMapper(k), types.KLineClosePriceMapper(k))
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last(0))
}

func (inc *NRR) LoadK(allKLines []types.KLine) {
	for _, k := range allKLines {
		inc.PushK(k)
	}
	inc.EmitUpdate(inc.Last(0))
}

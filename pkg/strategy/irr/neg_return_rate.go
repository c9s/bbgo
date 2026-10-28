package irr

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// NRR is the simple negative internal return rate over certain timeframe(interval)
//
// The values are the negative return rates, RankedValues are the ranks of the negative return rates in the
// ranking window divided by the ranking window, and ReturnValues are the return rates.
type NRR struct {
	*types.Float64Series

	RankingWindow int

	RankedValues floats.Slice
	ReturnValues floats.Slice

	window int
	delay  bool
	prices *types.Queue
}

// NewNRR creates the NRR stream of the given kline source.
// window = 2 means day-to-day return, previousClose/currentClose -1
// delay = false means use open/close-1 as D0 return
// delay = true means use the return of the close prices in the window as D1 return once the window is filled
func NewNRR(source indicatorv2.KLineSubscription, window, rankingWindow int, delay bool) *NRR {
	inc := &NRR{
		Float64Series: types.NewFloat64Series(),
		RankingWindow: rankingWindow,
		window:        window,
		delay:         delay,
		prices:        types.NewQueue(window),
	}

	source.AddSubscriber(func(k types.KLine) {
		inc.Update(k.Open.Float64(), k.Close.Float64())
	})

	return inc
}

func (inc *NRR) Update(openPrice, closePrice float64) {
	inc.prices.Update(closePrice)

	// D0
	nirr := (openPrice - closePrice) / openPrice
	irr := (closePrice - openPrice) / openPrice
	if inc.prices.Length() >= inc.window && inc.delay {
		// D1
		nirr = -1 * ((inc.prices.Last(0) / inc.prices.Index(inc.window-1)) - 1)
		irr = (inc.prices.Last(0) / inc.prices.Index(inc.window-1)) - 1
	}

	inc.Push(nirr) // neg ret here
	inc.truncate()

	inc.RankedValues.Push(inc.Rank(inc.RankingWindow).Last(0) / float64(inc.RankingWindow)) // ranked neg ret here
	inc.ReturnValues.Push(irr)

	inc.EmitUpdate(nirr)
}

func (inc *NRR) truncate() {
	// the rank reads the last RankingWindow values
	threshold := max(indicatorv2.MaxSliceSize, 4*inc.RankingWindow)
	size := max(indicatorv2.TruncateSize, inc.RankingWindow)

	inc.Slice = types.ShrinkSlice(inc.Slice, threshold, size)
	inc.RankedValues = types.ShrinkSlice(inc.RankedValues, threshold, size)
	inc.ReturnValues = types.ShrinkSlice(inc.ReturnValues, threshold, size)
}

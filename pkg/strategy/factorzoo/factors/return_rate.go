package factorzoo

import (
	"time"

	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// simply internal return rate over certain timeframe(interval)

type RR struct {
	*types.Float64Series

	window  int
	prices  *types.Queue
	EndTime time.Time
}

var _ types.SeriesExtend = &RR{}

// NewRR creates the return rate factor stream from the given kline source.
// A value is emitted for every kline, including the first one.
func NewRR(source indicatorv2.KLineSubscription, window int) *RR {
	inc := &RR{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		prices:        types.NewQueue(window),
	}
	source.AddSubscriber(inc.PushK)
	return inc
}

func (inc *RR) Update(price float64) {
	inc.prices.Update(price)
	irr := inc.prices.Last(0)/inc.prices.Index(1) - 1
	inc.PushAndEmit(irr)
	inc.Slice = types.ShrinkSlice(inc.Slice, indicatorv2.MaxSliceSize, indicatorv2.TruncateSize)
}

func (inc *RR) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(types.KLineClosePriceMapper(k))
	inc.EndTime = k.EndTime.Time()
}

// func calculateReturn(klines []types.KLine, window int, val KLineValueMapper) (float64, error) {
//	length := len(klines)
//	if length == 0 || length < window {
//		return 0.0, fmt.Errorf("insufficient elements for calculating VOL with window = %d", window)
//	}
//
//	rate := val(klines[length-1])/val(klines[length-2]) - 1
//
//	return rate, nil
// }

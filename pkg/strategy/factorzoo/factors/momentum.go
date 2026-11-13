package factorzoo

import (
	"fmt"
	"time"

	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// gap jump momentum
// if the gap between current open price and previous close price gets larger
// meaning an opening price jump was happened, the larger momentum we get is our alpha, MOM

type MOM struct {
	*types.Float64Series

	window int
	opens  *types.Queue
	closes *types.Queue

	EndTime time.Time
}

var _ types.SeriesExtend = &MOM{}

// NewMOM creates the gap jump momentum factor stream from the given kline source.
func NewMOM(source indicatorv2.KLineSubscription, window int) *MOM {
	inc := &MOM{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		opens:         types.NewQueue(window),
		closes:        types.NewQueue(window + 1),
	}
	source.AddSubscriber(inc.PushK)
	return inc
}

func (inc *MOM) Update(open, close float64) {
	inc.opens.Update(open)
	inc.closes.Update(close)
	if inc.opens.Length() >= inc.window && inc.closes.Length() >= inc.window {
		gap := inc.opens.Last(0)/inc.closes.Index(1) - 1
		inc.PushAndEmit(gap)
		inc.Slice = types.ShrinkSlice(inc.Slice, indicatorv2.MaxSliceSize, indicatorv2.TruncateSize)
	}
}

func (inc *MOM) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(k.Open.Float64(), k.Close.Float64())
	inc.EndTime = k.EndTime.Time()
}

func calculateMomentum(klines []types.KLine, window int, valA KLineValueMapper, valB KLineValueMapper) (float64, error) {
	length := len(klines)
	if length == 0 || length < window {
		return 0.0, fmt.Errorf("insufficient elements for calculating VOL with window = %d", window)
	}

	momentum := (1 - valA(klines[length-1])/valB(klines[length-1])) * -1

	return momentum, nil
}

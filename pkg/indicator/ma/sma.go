package ma

import (
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/store"
	"github.com/c9s/bbgo/pkg/types"
)

type SMA struct {
	store  *store.MarketDataStore
	Window int
}

func NewSMA(window int) *SMA {
	return &SMA{
		Window: window,
	}
}

func (i *SMA) handleUpdate(kline types.KLine) {
	klines, ok := i.store.KLineWindows[types.Interval(kline.Interval)]
	if !ok {
		return
	}

	if len(klines) < i.Window {
		return
	}

	// calculate ma
}

type IndicatorValue struct {
	Value float64
	Time  time.Time
}

func calculateMovingAverage(klines types.KLineWindow, period int) (values []IndicatorValue) {
	for idx := range klines[period:] {
		offset := idx + period
		sum := klines[offset-period : offset].ReduceClose()
		values = append(values, IndicatorValue{
			Time:  klines[offset].GetEndTime(),
			Value: math.Round(sum / float64(period)),
		})
	}
	return values
}

func (i *SMA) SubscribeStore(store *store.MarketDataStore) {
	i.store = store

	// register kline update callback
	// store.OnUpdate(i.handleUpdate)
}

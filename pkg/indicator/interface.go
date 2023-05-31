package indicator

import "github.com/c9s/bbgo/pkg/types"

type KLineWindowUpdater interface {
	OnKLineWindowUpdate(func(interval types.Interval, window types.KLineWindow))
}

type KLineClosedBinder interface {
	BindK(target KLineClosedEmitter, symbol string, interval types.Interval)
}

// KLineClosedEmitter is currently applied to the market data stream
// the market data stream emits the KLine closed event to the listeners.
type KLineClosedEmitter interface {
	OnKLineClosed(func(k types.KLine))
}

// KLinePusher provides an interface for API user to push kline value to the indicator.
// The indicator implements its own way to calculate the value from the given kline object.
type KLinePusher interface {
	PushK(k types.KLine)
}

// Simple is the simple indicator that only returns one float64 value
type Simple interface {
	KLinePusher
	Last(int) float64
	OnUpdate(f func(value float64))
}

type KLineCalculateUpdater interface {
	CalculateAndUpdate(allKLines []types.KLine)
}

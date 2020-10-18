package store

import (
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type MarketDataStore
type MarketDataStore struct {
	Symbol string

	// KLineWindows stores all loaded klines per interval
	KLineWindows map[types.Interval]types.KLineWindow `json:"-"`

	updateCallbacks []func(kline types.KLine)
}

func NewMarketDataStore(symbol string) *MarketDataStore {
	return &MarketDataStore{
		Symbol: symbol,

		// KLineWindows stores all loaded klines per interval
		KLineWindows: make(map[types.Interval]types.KLineWindow),
	}
}

func (store *MarketDataStore) BindStream(stream types.Stream) {
	stream.OnKLineClosed(store.handleKLineClosed)
}

func (store *MarketDataStore) handleKLineClosed(kline types.KLine) {
	if kline.Symbol == store.Symbol {
		store.AddKLine(kline)
	}
}

func (store *MarketDataStore) AddKLine(kline types.KLine) {
	var interval = types.Interval(kline.Interval)
	var window = store.KLineWindows[interval]
	window.Add(kline)

	store.EmitUpdate(kline)
}

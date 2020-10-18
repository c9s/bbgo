package store

import (
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type KLineStore
type KLineStore struct {
	Symbol string

	// KLineWindows stores all loaded klines per interval
	KLineWindows map[types.Interval]types.KLineWindow `json:"-"`

	LastKLine types.KLine

	updateCallbacks []func(kline types.KLine)
}

func NewMarketDataStore(symbol string) *KLineStore {
	return &KLineStore{
		Symbol: symbol,

		// KLineWindows stores all loaded klines per interval
		KLineWindows: make(map[types.Interval]types.KLineWindow),
	}
}

func (store *KLineStore) BindStream(stream types.Stream) {
	stream.OnKLineClosed(store.handleKLineClosed)
}

func (store *KLineStore) handleKLineClosed(kline types.KLine) {
	if kline.Symbol == store.Symbol {
		store.AddKLine(kline)
	}
}

func (store *KLineStore) AddKLine(kline types.KLine) {
	var interval = types.Interval(kline.Interval)
	var window = store.KLineWindows[interval]
	window.Add(kline)

	store.LastKLine = kline

	store.EmitUpdate(kline)
}

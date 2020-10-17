package bbgo

import (
	"github.com/c9s/bbgo/pkg/types"
)

type Interval string

var Interval1m = Interval("1m")
var Interval5m = Interval("5m")
var Interval1h = Interval("1h")
var Interval1d = Interval("1d")

type KLineCallback func(kline types.KLine)

//go:generate callbackgen -type MarketDataStore
type MarketDataStore struct {
	Symbol string

	// KLineWindows stores all loaded klines per interval
	KLineWindows map[Interval]types.KLineWindow `json:"-"`

	updateCallbacks []KLineCallback
}

func NewMarketDataStore(symbol string) *MarketDataStore {
	return &MarketDataStore{
		Symbol: symbol,

		// KLineWindows stores all loaded klines per interval
		KLineWindows: make(map[Interval]types.KLineWindow),
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
	var interval = Interval(kline.Interval)
	var window = store.KLineWindows[interval]
	window.Add(kline)

	store.EmitUpdate(kline)
}

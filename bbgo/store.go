package bbgo

import (
	"github.com/c9s/bbgo/pkg/bbgo/types"
)

type Interval string

var Interval1m = Interval("1m")
var Interval5m = Interval("5m")
var Interval1h = Interval("1h")
var Interval1d = Interval("1d")

type MarketDataStore struct {
	// MaxChangeKLines stores the max change kline per interval
	MaxChangeKLines map[Interval]types.KLine `json:"-"`

	// KLineWindows stores all loaded klines per interval
	KLineWindows map[Interval]types.KLineWindow `json:"-"`
}

func NewMarketDataStore() *MarketDataStore {
	return &MarketDataStore{
		MaxChangeKLines: make(map[Interval]types.KLine),

		// KLineWindows stores all loaded klines per interval
		KLineWindows: make(map[Interval]types.KLineWindow),
	}
}

func (store *MarketDataStore) BindPrivateStream(stream *types.StandardPrivateStream) {
	stream.OnKLineClosed(store.handleKLineClosed)
}

func (store *MarketDataStore) handleKLineClosed(kline *types.KLine) {
	store.AddKLine(*kline)
}

func (store *MarketDataStore) AddKLine(kline types.KLine) {
	var interval = Interval(kline.Interval)

	var window = store.KLineWindows[interval]
	window.Add(kline)

	if _, ok := store.MaxChangeKLines[interval] ; ok {
		if kline.GetMaxChange() > store.MaxChangeKLines[interval].GetMaxChange() {
			store.MaxChangeKLines[interval] = kline
		}
	}
}

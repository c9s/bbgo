package bbgo

import (
	"github.com/c9s/bbgo/pkg/bbgo/types"
)

type Interval string

var Interval1m = Interval("1m")
var Interval5m = Interval("5m")
var Interval1h = Interval("1h")
var Interval1d = Interval("1d")

type KLineStore struct {
	// MaxKLines stores the max change kline per interval
	MaxKLines map[Interval]types.KLine `json:"-"`

	// KLineWindows stores all loaded klines per interval
	KLineWindows map[Interval]types.KLineWindow `json:"-"`
}

func NewKLineStore() *KLineStore {
	return &KLineStore{
		MaxKLines: make(map[Interval]types.KLine),

		// KLineWindows stores all loaded klines per interval
		KLineWindows: make(map[Interval]types.KLineWindow),
	}
}

func (store *KLineStore) BindPrivateStream(stream *types.StandardPrivateStream) {
	stream.OnKLineClosed(store.handleKLineClosed)
}

func (store *KLineStore) handleKLineClosed(kline *types.KLine) {
	store.AddKLine(*kline)
}

func (store *KLineStore) AddKLine(kline types.KLine) {
	var interval = Interval(kline.Interval)

	var window = store.KLineWindows[interval]
	window.Add(kline)

	if kline.GetMaxChange() > store.MaxKLines[interval].GetMaxChange() {
		store.MaxKLines[interval] = kline
	}
}

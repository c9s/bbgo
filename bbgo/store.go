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
	// MaxChanges stores the max change kline per interval
	MaxChanges map[Interval]types.KLine `json:"-"`

	// Windows stores all loaded klines per interval
	Windows map[Interval]types.KLineWindow `json:"-"`
}

func NewKLineStore() *KLineStore {
	return &KLineStore{
		MaxChanges: make(map[Interval]types.KLine),

		// Windows stores all loaded klines per interval
		Windows: make(map[Interval]types.KLineWindow),
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

	var window = store.Windows[interval]
	window.Add(kline)


	if _, ok := store.MaxChanges[interval] ; ok {
		if kline.GetMaxChange() > store.MaxChanges[interval].GetMaxChange() {
			store.MaxChanges[interval] = kline
		}
	}
}

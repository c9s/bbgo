package bbgo

import "github.com/c9s/bbgo/pkg/types"

const MaxNumOfKLines = 5_000
const MaxNumOfKLinesTruncate = 100

// MarketDataStore receives and maintain the public market data of a single symbol
//go:generate callbackgen -type MarketDataStore
type MarketDataStore struct {
	Symbol string

	// KLineWindows stores all loaded klines per interval
	KLineWindows map[types.Interval]*types.KLineWindow `json:"-"`

	kLineWindowUpdateCallbacks []func(interval types.Interval, klines types.KLineWindow)
	kLineClosedCallbacks       []func(k types.KLine)
}

func NewMarketDataStore(symbol string) *MarketDataStore {
	return &MarketDataStore{
		Symbol: symbol,

		// KLineWindows stores all loaded klines per interval
		KLineWindows: make(map[types.Interval]*types.KLineWindow, len(types.SupportedIntervals)), // 13 interval, 1s,1m,5m,15m,30m,1h,2h,4h,6h,12h,1d,3d,1w
	}
}

func (store *MarketDataStore) SetKLineWindows(windows map[types.Interval]*types.KLineWindow) {
	store.KLineWindows = windows
}

// KLinesOfInterval returns the kline window of the given interval
func (store *MarketDataStore) KLinesOfInterval(interval types.Interval) (kLines *types.KLineWindow, ok bool) {
	kLines, ok = store.KLineWindows[interval]
	return kLines, ok
}

func (store *MarketDataStore) BindStream(stream types.Stream) {
	stream.OnKLineClosed(store.handleKLineClosed)
}

func (store *MarketDataStore) handleKLineClosed(kline types.KLine) {
	if kline.Symbol != store.Symbol {
		return
	}

	store.AddKLine(kline)
}

func (store *MarketDataStore) AddKLine(k types.KLine) {
	window, ok := store.KLineWindows[k.Interval]
	if !ok {
		var tmp = make(types.KLineWindow, 0, 1000)
		store.KLineWindows[k.Interval] = &tmp
		window = &tmp
	}
	window.Add(k)

	if len(*window) > MaxNumOfKLines {
		*window = (*window)[MaxNumOfKLinesTruncate-1:]
	}

	store.EmitKLineClosed(k)
	store.EmitKLineWindowUpdate(k.Interval, *window)
}

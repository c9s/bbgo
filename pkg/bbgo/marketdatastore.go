package bbgo

import "github.com/c9s/bbgo/pkg/types"

// MarketDataStore receives and maintain the public market data
//go:generate callbackgen -type MarketDataStore
type MarketDataStore struct {
	Symbol string

	// KLineWindows stores all loaded klines per interval
	KLineWindows map[types.Interval]types.KLineWindow `json:"-"`

	LastKLine types.KLine

	kLineWindowUpdateCallbacks []func(interval types.Interval, kline types.KLineWindow)

	orderBook *types.StreamOrderBook

	orderBookUpdateCallbacks []func(orderBook *types.StreamOrderBook)
}

func NewMarketDataStore(symbol string) *MarketDataStore {
	return &MarketDataStore{
		Symbol: symbol,

		orderBook: types.NewStreamBook(symbol),

		// KLineWindows stores all loaded klines per interval
		KLineWindows: make(map[types.Interval]types.KLineWindow, len(types.SupportedIntervals)), // 12 interval, 1m,5m,15m,30m,1h,2h,4h,6h,12h,1d,3d,1w
	}
}

func (store *MarketDataStore) SetKLineWindows(windows map[types.Interval]types.KLineWindow) {
	store.KLineWindows = windows
}

func (store *MarketDataStore) OrderBook() types.OrderBook {
	return store.orderBook.Copy()
}

// KLinesOfInterval returns the kline window of the given interval
func (store *MarketDataStore) KLinesOfInterval(interval types.Interval) (kLines types.KLineWindow, ok bool) {
	kLines, ok = store.KLineWindows[interval]
	return kLines, ok
}

func (store *MarketDataStore) handleOrderBookUpdate(book types.OrderBook) {
	if book.Symbol != store.Symbol {
		return
	}

	store.orderBook.Update(book)

	store.EmitOrderBookUpdate(store.orderBook)
}

func (store *MarketDataStore) handleOrderBookSnapshot(book types.OrderBook) {
	if book.Symbol != store.Symbol {
		return
	}

	store.orderBook.Load(book)
}

func (store *MarketDataStore) BindStream(stream types.Stream) {
	stream.OnKLineClosed(store.handleKLineClosed)
	stream.OnBookSnapshot(store.handleOrderBookSnapshot)
	stream.OnBookUpdate(store.handleOrderBookUpdate)

	store.orderBook.BindStream(stream)
}

func (store *MarketDataStore) handleKLineClosed(kline types.KLine) {
	if kline.Symbol != store.Symbol {
		return
	}

	store.AddKLine(kline)
}

func (store *MarketDataStore) AddKLine(kline types.KLine) {
	window := store.KLineWindows[kline.Interval]
	window.Add(kline)
	store.KLineWindows[kline.Interval] = window

	store.LastKLine = kline

	store.EmitKLineWindowUpdate(kline.Interval, window)
}

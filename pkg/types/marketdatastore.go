package types

const KLineWindowCapacityLimit = 5_000
const KLineWindowShrinkThreshold = KLineWindowCapacityLimit * 4 / 5
const KLineWindowShrinkSize = KLineWindowCapacityLimit / 5

// MarketDataStore receives and maintain the public market data of a single symbol
//
//go:generate callbackgen -type MarketDataStore
type MarketDataStore struct {
	Symbol string

	// KLineWindows stores all loaded klines per interval
	KLineWindows map[Interval]*KLineWindow `json:"-"`

	kLineWindowUpdateCallbacks []func(interval Interval, klines KLineWindow)
	kLineClosedCallbacks       []func(k KLine)
}

func NewMarketDataStore(symbol string) *MarketDataStore {
	return &MarketDataStore{
		Symbol: symbol,

		// KLineWindows stores all loaded klines per interval
		KLineWindows: make(map[Interval]*KLineWindow, len(SupportedIntervals)), // 13 interval, 1s,1m,5m,15m,30m,1h,2h,4h,6h,12h,1d,3d,1w
	}
}

func (store *MarketDataStore) SetKLineWindows(windows map[Interval]*KLineWindow) {
	store.KLineWindows = windows
}

// KLinesOfInterval returns the kline window of the given interval
func (store *MarketDataStore) KLinesOfInterval(interval Interval) (kLines *KLineWindow, ok bool) {
	kLines, ok = store.KLineWindows[interval]
	return kLines, ok
}

func (store *MarketDataStore) BindStream(stream Stream) {
	stream.OnKLineClosed(store.handleKLineClosed)
}

func (store *MarketDataStore) handleKLineClosed(kline KLine) {
	if kline.Symbol != store.Symbol {
		return
	}

	store.AddKLine(kline)
}

func (store *MarketDataStore) AddKLine(k KLine) {
	window, ok := store.KLineWindows[k.Interval]
	if !ok {
		var tmp = make(KLineWindow, 0, KLineWindowCapacityLimit)
		store.KLineWindows[k.Interval] = &tmp
		window = &tmp
	}
	window.Add(k)

	truncateKLineWindowIfNeeded(window)

	store.EmitKLineClosed(k)
	store.EmitKLineWindowUpdate(k.Interval, *window)
}

func truncateKLineWindowIfNeeded(window *KLineWindow) {
	*window = ShrinkSlice(*window, KLineWindowShrinkThreshold, KLineWindowShrinkSize)
}

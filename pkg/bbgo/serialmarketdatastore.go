package bbgo

import (
	"context"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	log "github.com/sirupsen/logrus"
)

type SerialMarketDataStore struct {
	*MarketDataStore
	UseMarketTrade           bool
	KLines                   map[types.Interval]*types.KLine
	MinInterval              types.Interval
	Subscription             []types.Interval
	o, h, l, c, v, qv, price fixedpoint.Value
	mu                       sync.Mutex
}

// @param symbol: symbol to trace on
// @param minInterval: unit interval, related to your signal timeframe
// @param useMarketTrade: if not assigned, default to false. if assigned to true, will use MarketTrade signal to generate klines
func NewSerialMarketDataStore(symbol string, minInterval types.Interval, useMarketTrade ...bool) *SerialMarketDataStore {
	return &SerialMarketDataStore{
		MarketDataStore: NewMarketDataStore(symbol),
		KLines:          make(map[types.Interval]*types.KLine),
		UseMarketTrade:  len(useMarketTrade) > 0 && useMarketTrade[0],
		Subscription:    []types.Interval{},
		MinInterval:     minInterval,
	}
}

func (store *SerialMarketDataStore) Subscribe(interval types.Interval) {
	// dedup
	for _, i := range store.Subscription {
		if i == interval {
			return
		}
	}
	store.Subscription = append(store.Subscription, interval)
}

func (store *SerialMarketDataStore) BindStream(ctx context.Context, stream types.Stream) {
	if store.UseMarketTrade {
		if IsBackTesting {
			log.Errorf("right now in backtesting, aggTrade event is not yet supported. Use OnKLineClosed instead.")
			stream.OnKLineClosed(store.handleKLineClosed)
			return
		}
		go store.tickerProcessor(ctx)
		stream.OnMarketTrade(store.handleMarketTrade)
	} else {
		stream.OnKLineClosed(store.handleKLineClosed)
	}
}

func (store *SerialMarketDataStore) handleKLineClosed(kline types.KLine) {
	store.AddKLine(kline)
}

func (store *SerialMarketDataStore) handleMarketTrade(trade types.Trade) {
	store.mu.Lock()
	store.price = trade.Price
	store.c = store.price
	if store.price.Compare(store.h) > 0 {
		store.h = store.price
	}
	if !store.l.IsZero() {
		if store.price.Compare(store.l) < 0 {
			store.l = store.price
		}
	} else {
		store.l = store.price
	}
	if store.o.IsZero() {
		store.o = store.price
	}
	store.v = store.v.Add(trade.Quantity)
	store.qv = store.qv.Add(trade.QuoteQuantity)
	store.mu.Unlock()
}

func (store *SerialMarketDataStore) tickerProcessor(ctx context.Context) {
	duration := store.MinInterval.Duration()
	relativeTime := time.Now().UnixNano() % int64(duration)
	waitTime := int64(duration) - relativeTime
	select {
	case <-time.After(time.Duration(waitTime)):
	case <-ctx.Done():
		return
	}
	intervalCloseTicker := time.NewTicker(duration)
	defer intervalCloseTicker.Stop()

	for {
		select {
		case time := <-intervalCloseTicker.C:
			kline := types.KLine{
				Symbol:    store.Symbol,
				StartTime: types.Time(time.Add(-1 * duration).Round(duration)),
				EndTime:   types.Time(time),
				Interval:  store.MinInterval,
				Closed:    true,
			}
			store.mu.Lock()
			if store.c.IsZero() {
				kline.Open = store.price
				kline.Close = store.price
				kline.High = store.price
				kline.Low = store.price
				kline.Volume = fixedpoint.Zero
				kline.QuoteVolume = fixedpoint.Zero
			} else {
				kline.Open = store.o
				kline.Close = store.c
				kline.High = store.h
				kline.Low = store.l
				kline.Volume = store.v
				kline.QuoteVolume = store.qv
				store.o = fixedpoint.Zero
				store.c = fixedpoint.Zero
				store.h = fixedpoint.Zero
				store.l = fixedpoint.Zero
				store.v = fixedpoint.Zero
				store.qv = fixedpoint.Zero
			}
			store.mu.Unlock()
			store.AddKLine(kline, true)
		case <-ctx.Done():
			return
		}
	}

}

func (store *SerialMarketDataStore) AddKLine(kline types.KLine, async ...bool) {
	if kline.Symbol != store.Symbol {
		return
	}
	// only consumes MinInterval
	if kline.Interval != store.MinInterval {
		return
	}
	// endtime
	duration := store.MinInterval.Duration()
	timestamp := kline.StartTime.Time().Add(duration)
	for _, val := range store.Subscription {
		k, ok := store.KLines[val]
		if !ok {
			k = &types.KLine{}
			k.Set(&kline)
			k.Interval = val
			k.Closed = false
			store.KLines[val] = k
		} else {
			k.Merge(&kline)
			k.Closed = false
		}
		if timestamp.Round(val.Duration()) == timestamp {
			k.Closed = true
			if len(async) > 0 && async[0] {
				go store.MarketDataStore.AddKLine(*k)
			} else {
				store.MarketDataStore.AddKLine(*k)
			}
			delete(store.KLines, val)
		}
	}
}

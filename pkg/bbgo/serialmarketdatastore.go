package bbgo

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

type SerialMarketDataStore struct {
	*MarketDataStore
	KLines       map[types.Interval]*types.KLine
	Subscription []types.Interval
}

func NewSerialMarketDataStore(symbol string) *SerialMarketDataStore {
	return &SerialMarketDataStore{
		MarketDataStore: NewMarketDataStore(symbol),
		KLines:          make(map[types.Interval]*types.KLine),
		Subscription:    []types.Interval{},
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

func (store *SerialMarketDataStore) BindStream(stream types.Stream) {
	stream.OnKLineClosed(store.handleKLineClosed)
}

func (store *SerialMarketDataStore) handleKLineClosed(kline types.KLine) {
	store.AddKLine(kline)
}

func (store *SerialMarketDataStore) AddKLine(kline types.KLine) {
	if kline.Symbol != store.Symbol {
		return
	}
	// only consumes kline1m
	if kline.Interval != types.Interval1m {
		return
	}
	// endtime in minutes
	timestamp := kline.StartTime.Time().Add(time.Minute)
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
		if timestamp.Truncate(val.Duration()) == timestamp {
			k.Closed = true
			store.MarketDataStore.AddKLine(*k)
			delete(store.KLines, val)
		}
	}
}

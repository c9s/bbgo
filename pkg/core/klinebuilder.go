package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

// kLineBuilder is a kline builder for a symbol
// It will accumulate trades and build klines
//
//go:generate callbackgen -type kLineBuilder
type kLineBuilder struct {
	symbol        string
	minInterval   types.Interval
	intervals     map[types.Interval]struct{}
	isBackTesting bool

	kLineCallbacks       []func(kline types.KLine)
	kLineClosedCallbacks []func(kline types.KLine)
	logger               *logrus.Entry

	//
	mu      sync.Mutex
	state   *kLineBuilderState
	running bool

	exchange types.Exchange
}

// NewKLineBuilder: constructor of kLineBuilder
//   - symbol: symbol to trace on
//   - minInterval: unit interval, related to your signal timeframe.
//     All the supported intervals of the binding stream should be multiple of this interval.
func NewKLineBuilder(symbol string, minInterval types.Interval, isBackTesting bool) *kLineBuilder {
	logger := logrus.WithField("symbol", symbol)
	// Interval.Duration() will panic on invalid value, ex: "-3m"
	_ = minInterval.Duration()

	return &kLineBuilder{
		symbol:        symbol,
		minInterval:   minInterval,
		isBackTesting: isBackTesting,
		logger:        logger,
		intervals:     make(map[types.Interval]struct{}),
		state:         NewKLineBuilderState(symbol),
	}
}

func (kb *kLineBuilder) Subscribe(interval types.Interval) {
	if interval.Duration()%kb.minInterval.Duration() != 0 {
		err := fmt.Errorf(
			"valid interval should be multiple of %s: %s is given",
			kb.minInterval,
			interval,
		)
		kb.logger.Error(err)
		panic(err)
	}
	kb.intervals[interval] = struct{}{}
}

func (kb *kLineBuilder) BindStream(stream types.Stream) {
	if kb.isBackTesting {
		stream.OnKLineClosed(kb.EmitKLineClosed)
		stream.OnKLine(kb.EmitKLine)
		return
	}
	stream.OnMarketTrade(kb.handleMarketTrade)
}

func (kb *kLineBuilder) SetExchange(exchange types.Exchange) {
	kb.exchange = exchange
}

func (kb *kLineBuilder) Run(ctx context.Context) {
	if kb.running {
		panic("can not run kline builder twice")
	}
	if len(kb.intervals) == 0 || kb.isBackTesting {
		return
	}
	kb.logger.Infof("kline updater started for %s (%+v)", kb.symbol, kb.intervals)
	kb.running = true
	go kb.run(ctx)
}

func (kb *kLineBuilder) handleMarketTrade(trade types.Trade) {
	// the market data stream may subscribe to trades of multiple symbols
	// so we need to check if the trade is for the symbol we are interested in
	if trade.Symbol != kb.symbol {
		return
	}
	kb.mu.Lock()
	defer kb.mu.Unlock()

	kb.state.AddTrade(trade)
}

func (kb *kLineBuilder) run(ctx context.Context) {
	startTime := time.Now()
	startTimeTruncated := startTime.Truncate(kb.minInterval.Duration())
	intervalDuration := kb.minInterval.Duration()
	if !startTimeTruncated.Equal(startTime) {
		// wait for the next interval
		// it's necessary since we want to collect trades as accurately as possible
		waitDuration := intervalDuration - startTime.Sub(startTimeTruncated)
		select {
		case <-time.After(waitDuration):
		case <-ctx.Done():
			return
		}
	}
	// start ticker
	ticker := time.NewTicker(intervalDuration)
	defer ticker.Stop()

	// reset the state
	// all kline of the intervals will be reset to the start time
	kb.mu.Lock()
	startTime = time.Now().Truncate(intervalDuration)
	for interval := range kb.intervals {
		kb.state.Reset(interval, types.Time(startTime))
	}
	kb.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return
		case tickTime := <-ticker.C:
			kb.tick(tickTime)
		}
	}
}

func (kb *kLineBuilder) tick(tickTime time.Time) {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	for interval := range kb.intervals {
		okForEmit := false
		kline, exist := kb.state.GetKLine(interval)
		if !exist {
			// should not happen
			continue
		}
		if kline.NumberOfTrades == 0 {
			// no trades in this interval
			// check if we have a last kline
			lastKline, found := kb.state.GetLastKLine(interval)
			oriStarTime := kline.StartTime
			if found {
				kline.Set(lastKline)
				kline.StartTime = oriStarTime
				okForEmit = true
			} else if kb.exchange != nil {
				// try to query kline from RESTful API
				queryStartTime := kline.StartTime.Time().Add(-kline.Interval.Duration())
				queryEndTime := queryStartTime.Add(2 * kline.Interval.Duration())
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				klines, err := kb.exchange.QueryKLines(
					ctx,
					kline.Symbol,
					kline.Interval, types.KLineQueryOptions{
						StartTime: &queryStartTime,
						EndTime:   &queryEndTime,
						Limit:     5,
					},
				)
				if err == nil && len(klines) > 0 {
					var timeDiff time.Duration
					var closestKline types.KLine
					for idx, k := range klines {
						if idx == 0 {
							closestKline = k
							timeDiff = k.StartTime.Time().Sub(kline.StartTime.Time()).Abs()
							continue
						}
						diff := k.StartTime.Time().Sub(kline.StartTime.Time()).Abs()
						if diff < timeDiff {
							timeDiff = diff
							closestKline = k
						}
					}
					kline.Set(&closestKline)
					kline.StartTime = oriStarTime
					okForEmit = true
				}
			}
		} else {
			okForEmit = true
		}

		// kline is not ready for emit, reset the state
		if !okForEmit {
			kb.state.Reset(interval, types.Time(tickTime))
			continue
		}
		// emit kline
		expectEndTime := kline.StartTime.Time().Add(kline.Interval.Duration())
		if kline.Interval == kb.minInterval {
			kline.Closed = true
		} else {
			kline.Closed = expectEndTime.Before(tickTime)
		}
		if kline.Closed {
			endTime := types.Time(expectEndTime.Add(-1 * time.Millisecond))
			kline.EndTime = endTime
			kb.state.Reset(kline.Interval, types.Time(expectEndTime))
			kb.EmitKLineClosed(*kline)
		} else {
			kb.EmitKLine(*kline)
		}
	}
}

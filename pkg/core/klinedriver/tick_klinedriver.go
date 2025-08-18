package klinedriver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	log "github.com/sirupsen/logrus"
)

type TickKLineDriver struct {
	symbol        string
	tickDuration  time.Duration
	intervals     map[types.Interval]struct{}
	intervalSlice []types.Interval

	kLineEmitter KLineEmitter

	mu           sync.Mutex
	builder      *KLineBuilder
	running      bool
	tradesBuffer []*types.Trade
}

type KLineEmitter interface {
	EmitKLine(types.KLine)
	EmitKLineClosed(types.KLine)
}

func NewTickKLineDriver(symbol string, tickDuration time.Duration) *TickKLineDriver {
	return &TickKLineDriver{
		symbol:        symbol,
		tickDuration:  tickDuration,
		intervals:     make(map[types.Interval]struct{}),
		intervalSlice: []types.Interval{},
		builder:       NewKLineBuilder(symbol),
		running:       false,
		tradesBuffer:  []*types.Trade{},
	}
}

func (t *TickKLineDriver) SetKLineEmitter(emitter KLineEmitter) {
	t.kLineEmitter = emitter
}

// SetRunning sets the running state of the driver.
// As for backtesting, use this method to set the start time of the driver.
func (t *TickKLineDriver) SetRunning(startTime time.Time) {
	// add intervals
	for interval := range t.intervals {
		// initialize the kline with the truncated start time
		startTimeTruncated := startTime.Truncate(interval.Duration())
		t.builder.AddInterval(interval, types.Time(startTimeTruncated))
	}
	t.running = true
}

func (t *TickKLineDriver) AddInterval(interval types.Interval) error {
	if interval.Duration() < t.tickDuration {
		return fmt.Errorf("interval %s can't be smaller than tick interval %s", interval, t.tickDuration)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.intervals[interval]; ok {
		return nil
	}
	t.intervals[interval] = struct{}{}
	t.intervalSlice = append(t.intervalSlice, interval)
	return nil
}

// AddTrade adds a trade to the kline driver.
func (t *TickKLineDriver) AddTrade(trade types.Trade) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// it's not running yet, buffer the trade
	if !t.running {
		t.tradesBuffer = append(t.tradesBuffer, &trade)
		return
	}
	// it's running, add buffered trades first
	if len(t.tradesBuffer) > 0 {
		for _, bufferTrade := range t.tradesBuffer {
			t.builder.AddTrade(*bufferTrade)
		}
		// empty the buffer
		t.tradesBuffer = []*types.Trade{}
	}
	t.builder.AddTrade(trade)
}

// Run starts the kline driver.
// The driver should only run once.
func (t *TickKLineDriver) Run(ctx context.Context) {
	go t.run(ctx)
}

func (t *TickKLineDriver) run(ctx context.Context) {
	if t.running {
		log.Debugf("kline driver %s-%s is already running", t.symbol, t.intervals)
		return
	}

	t.mu.Lock()
	// start ticker
	log.Debugf("starting kline driver: %s(%s) %v", t.symbol, t.tickDuration, t.intervalSlice)
	ticker := time.NewTicker(t.tickDuration)
	defer ticker.Stop()

	// set running state
	t.SetRunning(time.Now())

	// add buffered trades if any
	for _, trade := range t.tradesBuffer {
		t.builder.AddTrade(*trade)
	}
	t.mu.Unlock()

	// update loop
	for {
		select {
		case <-ctx.Done():
			return
		case tickTime := <-ticker.C:
			t.ProcessTick(tickTime)
		}
	}
}

// ProcessTick processes the tick time and emits kline events.
func (t *TickKLineDriver) ProcessTick(tickTime time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	kLinesMap := t.builder.Update(types.Time(tickTime))
	for _, interval := range t.intervalSlice {
		klines, ok := kLinesMap[interval]
		if !ok {
			// no kline for this interval, skip
			continue
		}
		for _, kline := range klines {
			if t.kLineEmitter != nil {
				if kline.Closed {
					// subtract 1ms: 08:01:00 -> 08:00:59.999, which is logically correct
					kline.EndTime = types.Time(kline.EndTime.Time().Truncate(time.Second).Add(-time.Millisecond * 1))
					t.kLineEmitter.EmitKLineClosed(*kline)
				} else {
					t.kLineEmitter.EmitKLine(*kline)
				}
			}
		}
	}
}

// Peek returns the current kline for all intervals.
// Useful for replay klines or backtesting
func (t *TickKLineDriver) Peek() map[types.Interval]types.KLine {
	klines := make(map[types.Interval]types.KLine)
	for interval, bucket := range t.builder.accBucketMap {
		klines[interval] = *(bucket.KLine)
	}
	return klines
}

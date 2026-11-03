// Package feed drives a public exchange stream and fans out regular klines
// and indicator/v2 tick/volume bars to subscribers.
package feed

import (
	"sync"

	"github.com/c9s/bbgo/pkg/types"
)

// KLineKey identifies a tracked (symbol, interval) kline series.
type KLineKey struct {
	Symbol   string
	Interval types.Interval
}

// KLineManager keeps a rolling window of the latest klines per (symbol, interval)
// and fans out live updates to subscribers.
type KLineManager struct {
	limit int

	mu   sync.RWMutex
	data map[KLineKey][]types.KLine
	subs map[KLineKey]map[chan types.KLine]struct{}
}

// NewKLineManager creates a KLineManager that keeps at most limit klines per series.
func NewKLineManager(limit int) *KLineManager {
	return &KLineManager{
		limit: limit,
		data:  make(map[KLineKey][]types.KLine),
		subs:  make(map[KLineKey]map[chan types.KLine]struct{}),
	}
}

// SetHistory seeds the in-memory window for a series, e.g. from a REST backfill query.
func (m *KLineManager) SetHistory(symbol string, interval types.Interval, kLines []types.KLine) {
	key := KLineKey{Symbol: symbol, Interval: interval}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = kLines
}

// handleKLine is registered as both the OnKLine and OnKLineClosed callback on the
// main stream: Binance only emits OnKLine for the still-forming bar and OnKLineClosed
// once it finalizes, so both are needed to keep the window live-updating.
func (m *KLineManager) handleKLine(k types.KLine) {
	key := KLineKey{Symbol: k.Symbol, Interval: k.Interval}

	m.mu.Lock()
	kLines := m.data[key]
	if n := len(kLines); n > 0 && kLines[n-1].StartTime.Equal(k.StartTime.Time()) {
		kLines[n-1] = k
	} else {
		kLines = append(kLines, k)
		if len(kLines) > m.limit {
			kLines = kLines[len(kLines)-m.limit:]
		}
	}
	m.data[key] = kLines

	var chans []chan types.KLine
	for ch := range m.subs[key] {
		chans = append(chans, ch)
	}
	m.mu.Unlock()

	for _, ch := range chans {
		select {
		case ch <- k:
		default:
		}
	}
}

// Snapshot returns a copy of the current in-memory window for a series.
func (m *KLineManager) Snapshot(symbol string, interval types.Interval) []types.KLine {
	key := KLineKey{Symbol: symbol, Interval: interval}

	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]types.KLine, len(m.data[key]))
	copy(out, m.data[key])
	return out
}

// Subscribe registers a channel that receives every live update for a series.
// The returned cancel function must be called to stop receiving updates.
func (m *KLineManager) Subscribe(symbol string, interval types.Interval) (<-chan types.KLine, func()) {
	key := KLineKey{Symbol: symbol, Interval: interval}
	ch := make(chan types.KLine, 32)

	m.mu.Lock()
	if m.subs[key] == nil {
		m.subs[key] = make(map[chan types.KLine]struct{})
	}
	m.subs[key][ch] = struct{}{}
	m.mu.Unlock()

	cancel := func() {
		m.mu.Lock()
		delete(m.subs[key], ch)
		m.mu.Unlock()
	}

	return ch, cancel
}

package types

import (
	"time"
)

// TradeRingBuffer uses a fixed capacity ring buffer to store trades.
type TradeRingBuffer struct {
	Capacity int
	Trades   []Trade
	Start    int // ring buffer start index
	Count    int // current number of stored trades
}

func NewTradeRingBuffer(capacity int) *TradeRingBuffer {
	return &TradeRingBuffer{
		Capacity: capacity,
		Trades:   make([]Trade, capacity),
	}
}

// Add adds a trade into the ring buffer.
func (b *TradeRingBuffer) Add(trade Trade) {
	if b.Count < b.Capacity {
		// If not full, add trade directly.
		idx := (b.Start + b.Count) % b.Capacity
		b.Trades[idx] = trade
		b.Count++
	} else {
		// If ring buffer is full, overwrite the oldest trade and update start index.
		b.Trades[b.Start] = trade
		b.Start = (b.Start + 1) % b.Capacity
	}
}

// Filter returns trades not before startTime, while updating the ring buffer.
func (b *TradeRingBuffer) Filter(startTime time.Time) []Trade {
	newStart := b.Start
	n := 0
	found := false
	// Find first trade with time after startTime.
	for i := 0; i < b.Count; i++ {
		idx := (b.Start + i) % b.Capacity
		if !b.Trades[idx].Time.Before(startTime) {
			newStart = idx
			n = b.Count - i
			found = true
			break
		}
	}

	if !found {
		b.Count = 0
		return nil
	}

	// Update ring buffer: set start and count.
	b.Start = newStart
	b.Count = n

	// Copy valid data to a new slice for return.
	res := make([]Trade, n)
	for i := 0; i < n; i++ {
		idx := (b.Start + i) % b.Capacity
		res[i] = b.Trades[idx]
	}
	return res
}

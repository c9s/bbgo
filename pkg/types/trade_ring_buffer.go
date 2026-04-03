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

	TradeCount int // total number of trades added
}

func NewTradeRingBuffer(capacity int) *TradeRingBuffer {
	return &TradeRingBuffer{
		Capacity: capacity,
		Trades:   make([]Trade, capacity),
	}
}

// Add adds a trade into the ring buffer.
func (b *TradeRingBuffer) Add(trade Trade) {
	b.TradeCount++
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

// TradeFrequency returns the number of trades per second in the buffer.
func (b *TradeRingBuffer) TradeFrequency() float64 {
	if b.Count < 2 {
		return 0
	}

	firstTrade := b.Trades[b.Start]
	lastTrade := b.Trades[(b.Start+b.Count-1)%b.Capacity]
	duration := lastTrade.Time.Time().Sub(firstTrade.Time.Time())
	if duration <= 0 {
		return 0
	}

	return float64(b.Count) / duration.Seconds()
}

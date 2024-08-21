package twap

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

// BboMonitor monitors the best bid and ask price and volume.
//
//go:generate callbackgen -type BboMonitor
type BboMonitor struct {
	Bid         types.PriceVolume
	Ask         types.PriceVolume
	UpdatedTime time.Time

	updateCallbacks []func(bid, ask types.PriceVolume)
}

func NewBboMonitor() *BboMonitor {
	return &BboMonitor{}
}

func (m *BboMonitor) UpdateFromBook(book *types.StreamOrderBook) bool {
	bestBid, ok1 := book.BestBid()
	bestAsk, ok2 := book.BestAsk()
	if !ok1 || !ok2 {
		return false
	}

	return m.Update(bestBid, bestAsk, book.LastUpdateTime())
}

func (m *BboMonitor) Update(bid, ask types.PriceVolume, t time.Time) bool {
	changed := false
	if m.Bid.Price.Compare(bid.Price) != 0 || m.Bid.Volume.Compare(bid.Volume) != 0 {
		changed = true
	}

	if m.Ask.Price.Compare(ask.Price) != 0 || m.Ask.Volume.Compare(ask.Volume) != 0 {
		changed = true
	}

	m.Bid = bid
	m.Ask = ask
	m.UpdatedTime = t

	if changed {
		m.EmitUpdate(bid, ask)
	}

	return changed
}

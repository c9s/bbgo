package types

import (
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// SliceOrderBook is a general order book structure which could be used
// for RESTful responses and websocket stream parsing
//go:generate callbackgen -type SliceOrderBook
type SliceOrderBook struct {
	Symbol string
	Bids   PriceVolumeSlice
	Asks   PriceVolumeSlice

	lastUpdateTime time.Time

	loadCallbacks   []func(book *SliceOrderBook)
	updateCallbacks []func(book *SliceOrderBook)
}

func NewSliceOrderBook(symbol string) *SliceOrderBook {
	return &SliceOrderBook{
		Symbol: symbol,
	}
}

func (b *SliceOrderBook) LastUpdateTime() time.Time {
	return b.lastUpdateTime
}

func (b *SliceOrderBook) Spread() (fixedpoint.Value, bool) {
	bestBid, ok := b.BestBid()
	if !ok {
		return fixedpoint.Zero, false
	}

	bestAsk, ok := b.BestAsk()
	if !ok {
		return fixedpoint.Zero, false
	}

	return bestAsk.Price.Sub(bestBid.Price), true
}

func (b *SliceOrderBook) BestBid() (PriceVolume, bool) {
	if len(b.Bids) == 0 {
		return PriceVolume{}, false
	}

	return b.Bids[0], true
}

func (b *SliceOrderBook) BestAsk() (PriceVolume, bool) {
	if len(b.Asks) == 0 {
		return PriceVolume{}, false
	}

	return b.Asks[0], true
}

func (b *SliceOrderBook) SideBook(sideType SideType) PriceVolumeSlice {
	switch sideType {

	case SideTypeBuy:
		return b.Bids

	case SideTypeSell:
		return b.Asks

	default:
		return nil
	}
}

func (b *SliceOrderBook) IsValid() (bool, error) {
	bid, hasBid := b.BestBid()
	ask, hasAsk := b.BestAsk()

	if !hasBid {
		return false, errors.New("empty bids")
	}

	if !hasAsk {
		return false, errors.New("empty asks")
	}

	if bid.Price.Compare(ask.Price) > 0 {
		return false, fmt.Errorf("bid price %s > ask price %s", bid.Price.String(), ask.Price.String())
	}

	return true, nil
}

func (b *SliceOrderBook) PriceVolumesBySide(side SideType) PriceVolumeSlice {
	switch side {

	case SideTypeBuy:
		return b.Bids.Copy()

	case SideTypeSell:
		return b.Asks.Copy()
	}

	return nil
}

func (b *SliceOrderBook) updateAsks(pvs PriceVolumeSlice) {
	for _, pv := range pvs {
		if pv.Volume.IsZero() {
			b.Asks = b.Asks.Remove(pv.Price, false)
		} else {
			b.Asks = b.Asks.Upsert(pv, false)
		}
	}
}

func (b *SliceOrderBook) updateBids(pvs PriceVolumeSlice) {
	for _, pv := range pvs {
		if pv.Volume.IsZero() {
			b.Bids = b.Bids.Remove(pv.Price, true)
		} else {
			b.Bids = b.Bids.Upsert(pv, true)
		}
	}
}

func (b *SliceOrderBook) update(book SliceOrderBook) {
	b.updateBids(book.Bids)
	b.updateAsks(book.Asks)
	b.lastUpdateTime = time.Now()
}

func (b *SliceOrderBook) Reset() {
	b.Bids = nil
	b.Asks = nil
}

func (b *SliceOrderBook) Load(book SliceOrderBook) {
	b.Reset()
	b.update(book)
	b.EmitLoad(b)
}

func (b *SliceOrderBook) Update(book SliceOrderBook) {
	b.update(book)
	b.EmitUpdate(b)
}

func (b *SliceOrderBook) Print() {
	fmt.Print(b.String())
}

func (b *SliceOrderBook) String() string {
	sb := strings.Builder{}

	sb.WriteString("BOOK ")
	sb.WriteString(b.Symbol)
	sb.WriteString("\n")

	if len(b.Asks) > 0 {
		sb.WriteString("ASKS:\n")
		for i := len(b.Asks) - 1; i >= 0; i-- {
			sb.WriteString("- ASK: ")
			sb.WriteString(b.Asks[i].String())
			sb.WriteString("\n")
		}
	}

	if len(b.Bids) > 0 {
		sb.WriteString("BIDS:\n")
		for _, bid := range b.Bids {
			sb.WriteString("- BID: ")
			sb.WriteString(bid.String())
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (b *SliceOrderBook) CopyDepth(limit int) OrderBook {
	var book SliceOrderBook
	book.Symbol = b.Symbol
	book.Bids = b.Bids.CopyDepth(limit)
	book.Asks = b.Asks.CopyDepth(limit)
	return &book
}

func (b *SliceOrderBook) Copy() OrderBook {
	var book SliceOrderBook
	book.Symbol = b.Symbol
	book.Bids = b.Bids.Copy()
	book.Asks = b.Asks.Copy()
	return &book
}

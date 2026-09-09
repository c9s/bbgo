package types

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types/skiplist"
)

const defaultSkipListSideBookLimit = 200

type priceLevelSkipList = skiplist.SkipList[fixedpoint.Value, fixedpoint.Value]

// priceAscending orders prices from low to high, which is the natural order of the ask side.
func priceAscending(a, b fixedpoint.Value) int {
	return a.Compare(b)
}

// priceDescending orders prices from high to low, which is the natural order of the bid side.
func priceDescending(a, b fixedpoint.Value) int {
	return b.Compare(a)
}

// SkipListOrderBook is an OrderBook implementation backed by a skip list.
// Both sides are kept sorted in their natural display order: bids descending and asks
// ascending, so the best price of each side is always the head of its skip list.
//
//go:generate callbackgen -type SkipListOrderBook
type SkipListOrderBook struct {
	Symbol string
	Bids   *priceLevelSkipList
	Asks   *priceLevelSkipList

	lastUpdateTime time.Time

	loadCallbacks   []func(book *SkipListOrderBook)
	updateCallbacks []func(book *SkipListOrderBook)
}

func NewSkipListOrderBook(symbol string) *SkipListOrderBook {
	return &SkipListOrderBook{
		Symbol: symbol,
		Bids:   skiplist.New[fixedpoint.Value, fixedpoint.Value](priceDescending),
		Asks:   skiplist.New[fixedpoint.Value, fixedpoint.Value](priceAscending),
	}
}

func (b *SkipListOrderBook) LastUpdateTime() time.Time {
	return b.lastUpdateTime
}

func (b *SkipListOrderBook) BestBid() (PriceVolume, bool) {
	// bids are sorted descending, so the highest bid is the first element
	price, volume, ok := b.Bids.Min()
	if !ok {
		return PriceVolume{}, false
	}

	return PriceVolume{Price: price, Volume: volume}, true
}

func (b *SkipListOrderBook) BestAsk() (PriceVolume, bool) {
	// asks are sorted ascending, so the lowest ask is the first element
	price, volume, ok := b.Asks.Min()
	if !ok {
		return PriceVolume{}, false
	}

	return PriceVolume{Price: price, Volume: volume}, true
}

func (b *SkipListOrderBook) Spread() (fixedpoint.Value, bool) {
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

func (b *SkipListOrderBook) IsValid() (bool, error) {
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

func (b *SkipListOrderBook) Load(book SliceOrderBook) {
	b.Reset()
	b.update(book)
	b.EmitLoad(b)
}

func (b *SkipListOrderBook) Update(book SliceOrderBook) {
	b.update(book)
	b.EmitUpdate(b)
}

func (b *SkipListOrderBook) Reset() {
	b.Bids = skiplist.New[fixedpoint.Value, fixedpoint.Value](priceDescending)
	b.Asks = skiplist.New[fixedpoint.Value, fixedpoint.Value](priceAscending)
}

func updateSkipListSide(sl *priceLevelSkipList, pvs PriceVolumeSlice) {
	for _, pv := range pvs {
		if pv.Volume.IsZero() {
			sl.Delete(pv.Price)
		} else {
			sl.Set(pv.Price, pv.Volume)
		}
	}
}

func (b *SkipListOrderBook) update(book SliceOrderBook) {
	updateSkipListSide(b.Bids, book.Bids)
	updateSkipListSide(b.Asks, book.Asks)
	b.lastUpdateTime = time.Now()
}

func copySkipList(src *priceLevelSkipList, cmp skiplist.Comparator[fixedpoint.Value], limit int) *priceLevelSkipList {
	dst := skiplist.New[fixedpoint.Value, fixedpoint.Value](cmp)
	n := 0
	src.Ascend(func(price, volume fixedpoint.Value) bool {
		dst.Set(price, volume)
		n++
		return !(limit > 0 && n >= limit)
	})
	return dst
}

func (b *SkipListOrderBook) Copy() OrderBook {
	return b.CopyDepth(0)
}

func (b *SkipListOrderBook) CopyDepth(limit int) OrderBook {
	book := NewSkipListOrderBook(b.Symbol)
	book.Bids = copySkipList(b.Bids, priceDescending, limit)
	book.Asks = copySkipList(b.Asks, priceAscending, limit)
	book.lastUpdateTime = b.lastUpdateTime
	return book
}

func (b *SkipListOrderBook) convertToPriceVolumeSlice(sl *priceLevelSkipList, limit int) PriceVolumeSlice {
	defCap := limit
	if defCap == 0 {
		if sl.Len() > 0 {
			defCap = sl.Len()
		} else {
			defCap = 50
		}
	}

	pvs := make(PriceVolumeSlice, 0, defCap)
	sl.Ascend(func(price, volume fixedpoint.Value) bool {
		pvs = append(pvs, PriceVolume{Price: price, Volume: volume})
		return !(limit > 0 && len(pvs) >= limit)
	})

	return pvs
}

func (b *SkipListOrderBook) SideBook(sideType SideType) PriceVolumeSlice {
	switch sideType {

	case SideTypeBuy:
		return b.convertToPriceVolumeSlice(b.Bids, defaultSkipListSideBookLimit)

	case SideTypeSell:
		return b.convertToPriceVolumeSlice(b.Asks, defaultSkipListSideBookLimit)

	default:
		return nil
	}
}

func (b *SkipListOrderBook) Print() {
	b.Asks.Ascend(func(price, volume fixedpoint.Value) bool {
		fmt.Printf("ask: %s x %s\n", price.String(), volume.String())
		return true
	})

	b.Bids.Ascend(func(price, volume fixedpoint.Value) bool {
		fmt.Printf("bid: %s x %s\n", price.String(), volume.String())
		return true
	})
}

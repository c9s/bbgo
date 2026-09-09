package types

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types/skiplist"
)

const defaultSkipListSideBookLimit = 200

// The skip list defaults (32 levels, promote with probability 1/2) are sized for lists of
// billions of entries, while an order book side holds thousands. That oversizing is not
// free: every Set and Delete allocates an update slice of maxLevel node pointers, so
// maxLevel multiplies the cost of the websocket update path.
//
// These values address roughly 4^10 (~1M) price levels, which leaves ample headroom over
// the deepest books we see, and measurably cut both time and allocation per update
// compared to the package defaults. See BenchmarkOrderBook_UpdateVolume.
const (
	defaultSkipListMaxLevel      = 10
	defaultSkipListSegmentLength = 4
)

func defaultSkipListSegmentLengths() []int {
	segLens := make([]int, defaultSkipListMaxLevel)
	for i := range segLens {
		segLens[i] = defaultSkipListSegmentLength
	}

	return segLens
}

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

	// segLens is kept so that Reset and CopyDepth rebuild the sides with the same shape
	// the book was created with, instead of silently falling back to the package defaults
	// on every snapshot load and stream reconnect.
	segLens []int

	lastUpdateTime time.Time

	loadCallbacks   []func(book *SkipListOrderBook)
	updateCallbacks []func(book *SkipListOrderBook)
}

// newPriceLevelSkipList builds one side of the book. It hands skiplist.New its own copy of
// segLens because New sanitizes the slice in place and then retains it.
func newPriceLevelSkipList(cmp skiplist.Comparator[fixedpoint.Value], segLens []int) *priceLevelSkipList {
	return skiplist.New[fixedpoint.Value, fixedpoint.Value](cmp, append([]int(nil), segLens...)...)
}

func NewSkipListOrderBook(symbol string) *SkipListOrderBook {
	return NewSkipListOrderBookWithSegmentLengths(symbol, defaultSkipListSegmentLengths())
}

// NewSkipListOrderBookWithSegmentLengths creates a book whose sides use the given per-level
// segment lengths, which control the maximum height of the skip list and the promotion
// probability at each level. It exists for tests and benchmarks that compare shapes; normal
// callers should use NewSkipListOrderBook and get the tuned defaults. An empty segLens
// falls back to those defaults.
func NewSkipListOrderBookWithSegmentLengths(symbol string, segLens []int) *SkipListOrderBook {
	if len(segLens) == 0 {
		segLens = defaultSkipListSegmentLengths()
	} else {
		segLens = append([]int(nil), segLens...)
	}

	return &SkipListOrderBook{
		Symbol:  symbol,
		Bids:    newPriceLevelSkipList(priceDescending, segLens),
		Asks:    newPriceLevelSkipList(priceAscending, segLens),
		segLens: segLens,
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
	if len(b.segLens) == 0 {
		b.segLens = defaultSkipListSegmentLengths()
	}

	b.Bids = newPriceLevelSkipList(priceDescending, b.segLens)
	b.Asks = newPriceLevelSkipList(priceAscending, b.segLens)
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

// copySkipListInto copies up to limit entries from src into dst in src's own order.
// A limit <= 0 copies everything. dst is written into rather than allocated here because
// constructing a skip list is expensive (it seeds its own rand source), so the lists the
// constructor already built are reused instead of being thrown away.
func copySkipListInto(dst, src *priceLevelSkipList, limit int) {
	n := 0
	src.Ascend(func(price, volume fixedpoint.Value) bool {
		dst.Set(price, volume)
		n++
		return !(limit > 0 && n >= limit)
	})
}

func (b *SkipListOrderBook) Copy() OrderBook {
	return b.CopyDepth(0)
}

func (b *SkipListOrderBook) CopyDepth(limit int) OrderBook {
	book := NewSkipListOrderBookWithSegmentLengths(b.Symbol, b.segLens)
	copySkipListInto(book.Bids, b.Bids, limit)
	copySkipListInto(book.Asks, b.Asks, limit)
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

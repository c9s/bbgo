package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// pvSlice builds a PriceVolumeSlice from alternating price/volume float pairs.
func pvSlice(priceVolumes ...float64) PriceVolumeSlice {
	pvs := make(PriceVolumeSlice, 0, len(priceVolumes)/2)
	for i := 0; i+1 < len(priceVolumes); i += 2 {
		pvs = append(pvs, PriceVolume{
			Price:  fixedpoint.NewFromFloat(priceVolumes[i]),
			Volume: fixedpoint.NewFromFloat(priceVolumes[i+1]),
		})
	}
	return pvs
}

func TestSkipListOrderBook_EmptyBook(t *testing.T) {
	book := NewSkipListOrderBook("BTCUSDT")
	bid, ok := book.BestBid()
	assert.False(t, ok)
	assert.Equal(t, fixedpoint.Zero, bid.Price)

	ask, ok := book.BestAsk()
	assert.False(t, ok)
	assert.Equal(t, fixedpoint.Zero, ask.Price)

	_, ok = book.Spread()
	assert.False(t, ok)

	valid, err := book.IsValid()
	assert.False(t, valid)
	assert.Error(t, err)
}

func TestSkipListOrderBook_Load(t *testing.T) {
	book := NewSkipListOrderBook("BTCUSDT")
	book.Load(SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids:   pvSlice(2800.0, 1, 2790.0, 2, 2795.0, 3),
		Asks:   pvSlice(2820.0, 1, 2810.0, 2, 2815.0, 3),
	})

	bid, ok := book.BestBid()
	assert.True(t, ok)
	assert.Equal(t, fixedpoint.NewFromFloat(2800.0), bid.Price)
	assert.Equal(t, fixedpoint.NewFromFloat(1), bid.Volume)

	ask, ok := book.BestAsk()
	assert.True(t, ok)
	assert.Equal(t, fixedpoint.NewFromFloat(2810.0), ask.Price)
	assert.Equal(t, fixedpoint.NewFromFloat(2), ask.Volume)

	spread, ok := book.Spread()
	assert.True(t, ok)
	assert.Equal(t, fixedpoint.NewFromFloat(10.0), spread)

	valid, err := book.IsValid()
	assert.True(t, valid)
	assert.NoError(t, err)

	// bids are returned in descending order, asks in ascending order
	assert.Equal(t, pvSlice(2800, 1, 2795, 3, 2790, 2), book.SideBook(SideTypeBuy))
	assert.Equal(t, pvSlice(2810, 2, 2815, 3, 2820, 1), book.SideBook(SideTypeSell))
}

func TestSkipListOrderBook_LoadResetsPreviousLevels(t *testing.T) {
	book := NewSkipListOrderBook("BTCUSDT")
	book.Load(SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids:   pvSlice(2800.0, 1),
		Asks:   pvSlice(2810.0, 1),
	})

	book.Load(SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids:   pvSlice(2700.0, 1),
		Asks:   pvSlice(2710.0, 1),
	})

	assert.Equal(t, pvSlice(2700, 1), book.SideBook(SideTypeBuy))
	assert.Equal(t, pvSlice(2710, 1), book.SideBook(SideTypeSell))
}

func TestSkipListOrderBook_UpdateAndDelete(t *testing.T) {
	book := NewSkipListOrderBook("BTCUSDT")
	book.Load(SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids:   pvSlice(2800.0, 1, 2790.0, 2),
		Asks:   pvSlice(2810.0, 1, 2820.0, 2),
	})

	// zero volume removes the price level, non-zero volume replaces it
	book.Update(SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids:   pvSlice(2800.0, 0, 2790.0, 5),
		Asks:   pvSlice(2810.0, 0, 2820.0, 5),
	})

	bid, ok := book.BestBid()
	assert.True(t, ok)
	assert.Equal(t, fixedpoint.NewFromFloat(2790.0), bid.Price)
	assert.Equal(t, fixedpoint.NewFromFloat(5), bid.Volume)

	ask, ok := book.BestAsk()
	assert.True(t, ok)
	assert.Equal(t, fixedpoint.NewFromFloat(2820.0), ask.Price)
	assert.Equal(t, fixedpoint.NewFromFloat(5), ask.Volume)

	book.Update(SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids:   pvSlice(2790.0, 0),
		Asks:   pvSlice(2820.0, 0),
	})

	_, ok = book.BestBid()
	assert.False(t, ok)
	_, ok = book.BestAsk()
	assert.False(t, ok)
}

func TestSkipListOrderBook_Reset(t *testing.T) {
	book := NewSkipListOrderBook("BTCUSDT")
	book.Load(SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids:   pvSlice(2800.0, 1),
		Asks:   pvSlice(2810.0, 1),
	})

	book.Reset()
	assert.Equal(t, 0, book.Bids.Len())
	assert.Equal(t, 0, book.Asks.Len())
}

func TestSkipListOrderBook_Copy(t *testing.T) {
	book := NewSkipListOrderBook("BTCUSDT")
	book.Load(SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids:   pvSlice(2800.0, 1, 2790.0, 2, 2780.0, 3),
		Asks:   pvSlice(2810.0, 1, 2820.0, 2, 2830.0, 3),
	})

	c := book.Copy()
	assert.Equal(t, book.SideBook(SideTypeBuy), c.SideBook(SideTypeBuy))
	assert.Equal(t, book.SideBook(SideTypeSell), c.SideBook(SideTypeSell))

	// mutating the copy must not affect the original
	c.Update(SliceOrderBook{Symbol: "BTCUSDT", Bids: pvSlice(2800.0, 0)})
	assert.Equal(t, 3, book.Bids.Len())

	d := book.CopyDepth(2)
	assert.Equal(t, pvSlice(2800, 1, 2790, 2), d.SideBook(SideTypeBuy))
	assert.Equal(t, pvSlice(2810, 1, 2820, 2), d.SideBook(SideTypeSell))
}

func TestNewMutexOrderBook_SkipListBackend(t *testing.T) {
	t.Setenv("ENABLE_SKIPLIST_ORDERBOOK", "true")

	book := NewMutexOrderBook("BTCUSDT", ExchangeBinance)
	assert.IsType(t, &SkipListOrderBook{}, book.orderBook)
}

func TestSkipListOrderBook_ResetPreservesSegmentLengths(t *testing.T) {
	segLens := []int{2, 2, 2}
	book := NewSkipListOrderBookWithSegmentLengths("BTCUSDT", segLens)

	// Load resets both sides, which must not fall back to the package defaults
	book.Load(SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids:   pvSlice(2800, 1),
		Asks:   pvSlice(2810, 1),
	})
	assert.Equal(t, segLens, book.segLens)
	assert.Equal(t, 3, book.Bids.MaxLevel())
	assert.Equal(t, 3, book.Asks.MaxLevel())

	book.Reset()
	assert.Equal(t, 3, book.Bids.MaxLevel())
	assert.Equal(t, 3, book.Asks.MaxLevel())
}

func TestSkipListOrderBook_CopyPreservesSegmentLengths(t *testing.T) {
	book := NewSkipListOrderBookWithSegmentLengths("BTCUSDT", []int{2, 2, 2})
	book.Load(SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids:   pvSlice(2800, 1, 2790, 2),
		Asks:   pvSlice(2810, 1, 2820, 2),
	})

	c := book.CopyDepth(1).(*SkipListOrderBook)
	assert.Equal(t, 3, c.Bids.MaxLevel())
	assert.Equal(t, 3, c.Asks.MaxLevel())
}

func TestSkipListOrderBook_DefaultSegmentLengths(t *testing.T) {
	book := NewSkipListOrderBook("BTCUSDT")
	assert.Equal(t, defaultSkipListMaxLevel, book.Bids.MaxLevel())
	assert.Equal(t, defaultSkipListMaxLevel, book.Asks.MaxLevel())

	// the caller's slice must not be retained or mutated by the constructor
	segLens := []int{1, 1}
	custom := NewSkipListOrderBookWithSegmentLengths("BTCUSDT", segLens)
	assert.Equal(t, []int{1, 1}, segLens, "constructor must not mutate the caller's slice")
	assert.Equal(t, 2, custom.Bids.MaxLevel())

	// an empty segLens falls back to the tuned defaults rather than an unusable book
	fallback := NewSkipListOrderBookWithSegmentLengths("BTCUSDT", nil)
	assert.Equal(t, defaultSkipListMaxLevel, fallback.Bids.MaxLevel())
}

// a book built with the tuned defaults must behave identically to one built with the
// skiplist package defaults, at a depth well past what a real book reaches
func TestSkipListOrderBook_SegmentLengthsDoNotChangeOrdering(t *testing.T) {
	snapshot := makeOrderBookSnapshot("BTCUSDT", 8000)

	ref := NewSkipListOrderBookWithSegmentLengths("BTCUSDT", segmentLengthsForTest(32, 2))
	ref.Load(snapshot)

	book := NewSkipListOrderBook("BTCUSDT")
	book.Load(snapshot)

	assert.Equal(t, 8000, book.Bids.Len())
	assert.Equal(t, 8000, book.Asks.Len())
	assert.Equal(t, ref.SideBook(SideTypeBuy), book.SideBook(SideTypeBuy))
	assert.Equal(t, ref.SideBook(SideTypeSell), book.SideBook(SideTypeSell))
}

func segmentLengthsForTest(maxLevel, seg int) []int {
	segLens := make([]int, maxLevel)
	for i := range segLens {
		segLens[i] = seg
	}
	return segLens
}

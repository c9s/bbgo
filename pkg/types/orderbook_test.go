package types

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func prepareOrderBookBenchmarkData() (asks, bids PriceVolumeSlice) {
	for p := 0.0; p < 1000.0; p++ {
		asks = append(asks, PriceVolume{fixedpoint.NewFromFloat(1000 + p), fixedpoint.One})
		bids = append(bids, PriceVolume{fixedpoint.NewFromFloat(1000 - 0.1 - p), fixedpoint.One})
	}
	return
}

func BenchmarkOrderBook_Load(b *testing.B) {
	var asks, bids = prepareOrderBookBenchmarkData()
	for p := 0.0; p < 1000.0; p++ {
		asks = append(asks, PriceVolume{fixedpoint.NewFromFloat(1000 + p), fixedpoint.One})
		bids = append(bids, PriceVolume{fixedpoint.NewFromFloat(1000 - 0.1 - p), fixedpoint.One})
	}

	b.Run("RBTOrderBook", func(b *testing.B) {
		book := NewRBOrderBook("ETHUSDT")
		for i := 0; i < b.N; i++ {
			for _, ask := range asks {
				book.Asks.Upsert(ask.Price, ask.Volume)
			}
			for _, bid := range bids {
				book.Bids.Upsert(bid.Price, bid.Volume)
			}
		}
	})

	b.Run("OrderBook", func(b *testing.B) {
		book := &SliceOrderBook{}
		for i := 0; i < b.N; i++ {
			for _, ask := range asks {
				book.Asks = book.Asks.Upsert(ask, false)
			}
			for _, bid := range bids {
				book.Bids = book.Bids.Upsert(bid, true)
			}
		}
	})
}

func BenchmarkOrderBook_UpdateAndInsert(b *testing.B) {
	var asks, bids = prepareOrderBookBenchmarkData()
	for p := 0.0; p < 1000.0; p += 2 {
		asks = append(asks, PriceVolume{fixedpoint.NewFromFloat(1000 + p), fixedpoint.One})
		bids = append(bids, PriceVolume{fixedpoint.NewFromFloat(1000 - 0.1 - p), fixedpoint.One})
	}

	rbBook := NewRBOrderBook("ETHUSDT")
	for _, ask := range asks {
		rbBook.Asks.Upsert(ask.Price, ask.Volume)
	}
	for _, bid := range bids {
		rbBook.Bids.Upsert(bid.Price, bid.Volume)
	}

	b.Run("RBTOrderBook", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var price = fixedpoint.NewFromFloat(rand.Float64() * 2000.0)
			if price.Compare(fixedpoint.NewFromInt(1000)) >= 0 {
				rbBook.Asks.Upsert(price, fixedpoint.One)
			} else {
				rbBook.Bids.Upsert(price, fixedpoint.One)
			}
		}
	})

	sliceBook := &SliceOrderBook{}
	for i := 0; i < b.N; i++ {
		for _, ask := range asks {
			sliceBook.Asks = sliceBook.Asks.Upsert(ask, false)
		}
		for _, bid := range bids {
			sliceBook.Bids = sliceBook.Bids.Upsert(bid, true)
		}
	}
	b.Run("OrderBook", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var price = fixedpoint.NewFromFloat(rand.Float64() * 2000.0)
			if price.Compare(fixedpoint.NewFromFloat(1000)) >= 0 {
				sliceBook.Asks = sliceBook.Asks.Upsert(PriceVolume{Price: price, Volume: fixedpoint.NewFromFloat(1)}, false)
			} else {
				sliceBook.Bids = sliceBook.Bids.Upsert(PriceVolume{Price: price, Volume: fixedpoint.NewFromFloat(1)}, true)
			}
		}
	})
}

func TestOrderBook_IsValid(t *testing.T) {
	ob := SliceOrderBook{
		Bids: PriceVolumeSlice{
			{fixedpoint.NewFromFloat(100.0), fixedpoint.NewFromFloat(1.5)},
			{fixedpoint.NewFromFloat(90.0), fixedpoint.NewFromFloat(2.5)},
		},

		Asks: PriceVolumeSlice{
			{fixedpoint.NewFromFloat(110.0), fixedpoint.NewFromFloat(1.5)},
			{fixedpoint.NewFromFloat(120.0), fixedpoint.NewFromFloat(2.5)},
		},
	}

	isValid, err := ob.IsValid()
	assert.True(t, isValid)
	assert.NoError(t, err)

	ob.Bids = nil
	isValid, err = ob.IsValid()
	assert.False(t, isValid)
	assert.EqualError(t, err, "empty bids")

	ob.Bids = PriceVolumeSlice{
		{fixedpoint.NewFromFloat(80000.0), fixedpoint.NewFromFloat(1.5)},
		{fixedpoint.NewFromFloat(120.0), fixedpoint.NewFromFloat(2.5)},
	}

	ob.Asks = nil
	isValid, err = ob.IsValid()
	assert.False(t, isValid)
	assert.EqualError(t, err, "empty asks")

	ob.Asks = PriceVolumeSlice{
		{fixedpoint.NewFromFloat(100.0), fixedpoint.NewFromFloat(1.5)},
		{fixedpoint.NewFromFloat(90.0), fixedpoint.NewFromFloat(2.5)},
	}
	isValid, err = ob.IsValid()
	assert.False(t, isValid)
	assert.EqualError(t, err, "bid price 80000 > ask price 100")
}

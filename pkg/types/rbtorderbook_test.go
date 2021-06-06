package types

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/stretchr/testify/assert"
)

func TestRBOrderBook_EmptyBook(t *testing.T) {
	book := NewRBOrderBook("BTCUSDT")
	bid, ok := book.BestBid()
	assert.False(t, ok)
	assert.Equal(t, fixedpoint.Value(0), bid.Price)

	ask, ok := book.BestAsk()
	assert.False(t, ok)
	assert.Equal(t, fixedpoint.Value(0), ask.Price)
}

func TestRBOrderBook_Load(t *testing.T) {
	book := NewRBOrderBook("BTCUSDT")

	book.Load(SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids: PriceVolumeSlice{
			{Price: fixedpoint.NewFromFloat(2800.0), Volume: fixedpoint.NewFromFloat(1.0)},
		},
		Asks: PriceVolumeSlice{
			{Price: fixedpoint.NewFromFloat(2810.0), Volume: fixedpoint.NewFromFloat(1.0)},
		},
	})

	bid, ok := book.BestBid()
	assert.True(t, ok)
	assert.Equal(t, fixedpoint.NewFromFloat(2800.0), bid.Price)

	ask, ok := book.BestAsk()
	assert.True(t, ok)
	assert.Equal(t, fixedpoint.NewFromFloat(2810.0), ask.Price)
}

func TestRBOrderBook_LoadAndDelete(t *testing.T) {
	book := NewRBOrderBook("BTCUSDT")

	book.Load(SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids: PriceVolumeSlice{
			{Price: fixedpoint.NewFromFloat(2800.0), Volume: fixedpoint.NewFromFloat(1.0)},
		},
		Asks: PriceVolumeSlice{
			{Price: fixedpoint.NewFromFloat(2810.0), Volume: fixedpoint.NewFromFloat(1.0)},
		},
	})

	bid, ok := book.BestBid()
	assert.True(t, ok)
	assert.Equal(t, fixedpoint.NewFromFloat(2800.0), bid.Price)

	ask, ok := book.BestAsk()
	assert.True(t, ok)
	assert.Equal(t, fixedpoint.NewFromFloat(2810.0), ask.Price)

	book.Load(SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids: PriceVolumeSlice{
			{Price: fixedpoint.NewFromFloat(2800.0), Volume: 0},
		},
		Asks: PriceVolumeSlice{
			{Price: fixedpoint.NewFromFloat(2810.0), Volume: 0},
		},
	})

	bid, ok = book.BestBid()
	assert.False(t, ok)
	ask, ok = book.BestAsk()
	assert.False(t, ok)
}

package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSliceOrderBook_CopyDepth(t *testing.T) {
	b := &SliceOrderBook{
		Bids: PriceVolumeSlice{
			{Price: number(0.119), Volume: number(100.0)},
			{Price: number(0.118), Volume: number(100.0)},
			{Price: number(0.117), Volume: number(100.0)},
			{Price: number(0.116), Volume: number(100.0)},
		},
		Asks: PriceVolumeSlice{
			{Price: number(0.120), Volume: number(100.0)},
			{Price: number(0.121), Volume: number(100.0)},
			{Price: number(0.122), Volume: number(100.0)},
		},
	}

	copied := b.CopyDepth(0)
	assert.Equal(t, 3, len(copied.SideBook(SideTypeSell)))
	assert.Equal(t, 4, len(copied.SideBook(SideTypeBuy)))
}

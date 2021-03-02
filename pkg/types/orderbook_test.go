package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestOrderBook_IsValid(t *testing.T) {
	ob := OrderBook{
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
	assert.EqualError(t, err, "bid price 80000.000000 > ask price 100.000000")
}

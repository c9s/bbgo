package xmaker

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_aggregatePrice(t *testing.T) {
	bids := types.PriceVolumeSlice{
		{
			Price:  fixedpoint.NewFromFloat(1000.0),
			Volume: fixedpoint.NewFromFloat(1.0),
		},
		{
			Price:  fixedpoint.NewFromFloat(1200.0),
			Volume: fixedpoint.NewFromFloat(1.0),
		},
		{
			Price:  fixedpoint.NewFromFloat(1400.0),
			Volume: fixedpoint.NewFromFloat(1.0),
		},
	}

	aggregatedPrice1 := aggregatePrice(bids, fixedpoint.NewFromFloat(0.5))
	assert.Equal(t, fixedpoint.NewFromFloat(1000.0), aggregatedPrice1)

	aggregatedPrice2 := aggregatePrice(bids, fixedpoint.NewFromInt(1))
	assert.Equal(t, fixedpoint.NewFromFloat(1000.0), aggregatedPrice2)

	aggregatedPrice3 := aggregatePrice(bids, fixedpoint.NewFromInt(2))
	assert.Equal(t, fixedpoint.NewFromFloat(1100.0), aggregatedPrice3)

}

package bbgo

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestPosition(t *testing.T) {
	trades := []types.Trade{
		{
			Side:          types.SideTypeBuy,
			Price:         1000.0,
			Quantity:      0.01,
			QuoteQuantity: 1000.0 * 0.01,
		},
		{
			Side:          types.SideTypeBuy,
			Price:         2000.0,
			Quantity:      0.03,
			QuoteQuantity: 2000.0 * 0.03,
		},
	}

	pos := Position{}
	for _, trade := range trades {
		pos.AddTrade(trade)
	}

	assert.Equal(t, fixedpoint.NewFromFloat(-70.0), pos.Quote)
	assert.Equal(t, fixedpoint.NewFromFloat(0.04), pos.Base)
	assert.Equal(t, fixedpoint.NewFromFloat((1000.0*0.01+2000.0*0.03)/0.04), pos.AverageCost)
}

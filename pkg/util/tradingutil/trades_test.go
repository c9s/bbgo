package tradingutil

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var number = fixedpoint.MustNewFromString

func Test_CollectTradeFee(t *testing.T) {
	trades := []types.Trade{
		{
			ID:            1,
			Price:         number("21000"),
			Quantity:      number("0.001"),
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeBuy,
			Fee:           number("0.00001"),
			FeeCurrency:   "BTC",
			FeeDiscounted: false,
		},
		{
			ID:            2,
			Price:         number("21200"),
			Quantity:      number("0.001"),
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeBuy,
			Fee:           number("0.00002"),
			FeeCurrency:   "BTC",
			FeeDiscounted: false,
		},
	}

	fees := CollectTradeFee(trades)
	assert.NotNil(t, fees)
	assert.Equal(t, number("0.00003"), fees["BTC"])
}

package dca2

import (
	"testing"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestGenerateTakeProfitOrder(t *testing.T) {
	assert := assert.New(t)

	strategy := newTestStrategy()

	position := types.NewPositionFromMarket(strategy.Market)
	position.AddTrade(types.Trade{
		Symbol:        "BTCUSDT",
		Side:          types.SideTypeBuy,
		Price:         Number("28500"),
		Quantity:      Number("1"),
		QuoteQuantity: Number("28500"),
		Fee:           Number("0.0015"),
		FeeCurrency:   strategy.Market.BaseCurrency,
	})

	o := generateTakeProfitOrder(strategy.Market, strategy.TakeProfitRatio, position, strategy.OrderGroupID)
	assert.Equal(Number("31397.09"), o.Price)
	assert.Equal(Number("0.9985"), o.Quantity)
	assert.Equal(types.SideTypeSell, o.Side)
	assert.Equal(strategy.Symbol, o.Symbol)

	position.AddTrade(types.Trade{
		Side:          types.SideTypeBuy,
		Price:         Number("27000"),
		Quantity:      Number("0.5"),
		QuoteQuantity: Number("13500"),
		Fee:           Number("0.00075"),
		FeeCurrency:   strategy.Market.BaseCurrency,
	})
	o = generateTakeProfitOrder(strategy.Market, strategy.TakeProfitRatio, position, strategy.OrderGroupID)
	assert.Equal(Number("30846.26"), o.Price)
	assert.Equal(Number("1.49775"), o.Quantity)
	assert.Equal(types.SideTypeSell, o.Side)
	assert.Equal(strategy.Symbol, o.Symbol)

}

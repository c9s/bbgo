package backtest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func newLimitOrder(symbol string, side types.SideType, price, quantity float64) types.SubmitOrder {
	return types.SubmitOrder{
		Symbol:      symbol,
		Side:        side,
		Type:        types.OrderTypeLimit,
		Quantity:    fixedpoint.NewFromFloat(quantity),
		Price:       fixedpoint.NewFromFloat(price),
		TimeInForce: types.TimeInForceGTC,
	}
}

func TestSimplePriceMatching_LimitOrder(t *testing.T) {
	account := &types.Account{
		MakerFeeRate: fixedpoint.NewFromFloat(0.075 * 0.01),
		TakerFeeRate: fixedpoint.NewFromFloat(0.075 * 0.01),
	}

	account.UpdateBalances(types.BalanceMap{
		"USDT": {Currency: "USDT", Available: fixedpoint.NewFromFloat(1000000.0)},
		"BTC":  {Currency: "BTC", Available: fixedpoint.NewFromFloat(100.0)},
	})

	market := types.Market{
		Symbol:          "BTCUSDT",
		PricePrecision:  8,
		VolumePrecision: 8,
		QuoteCurrency:   "USDT",
		BaseCurrency:    "BTC",
		MinNotional:     fixedpoint.MustNewFromString("0.001"),
		MinAmount:       fixedpoint.MustNewFromString("10.0"),
		MinQuantity:     fixedpoint.MustNewFromString("0.001"),
	}

	engine := &SimplePriceMatching{
		CurrentTime: time.Now(),
		Account:     account,
		Market:      market,
	}

	for i := 0; i < 5; i++ {
		_, _, err := engine.PlaceOrder(newLimitOrder("BTCUSDT", types.SideTypeBuy, 8000.0-float64(i), 1.0))
		assert.NoError(t, err)
	}
	assert.Len(t, engine.bidOrders, 5)
	assert.Len(t, engine.askOrders, 0)

	for i := 0; i < 5; i++ {
		_, _, err := engine.PlaceOrder(newLimitOrder("BTCUSDT", types.SideTypeSell, 9000.0+float64(i), 1.0))
		assert.NoError(t, err)
	}
	assert.Len(t, engine.bidOrders, 5)
	assert.Len(t, engine.askOrders, 5)

	closedOrders, trades := engine.SellToPrice(fixedpoint.NewFromFloat(8100.0))
	assert.Len(t, closedOrders, 0)
	assert.Len(t, trades, 0)

	closedOrders, trades = engine.SellToPrice(fixedpoint.NewFromFloat(8000.0))
	assert.Len(t, closedOrders, 1)
	assert.Len(t, trades, 1)
	for _, trade := range trades {
		assert.True(t, trade.IsBuyer)
	}

	for _, o := range closedOrders {
		assert.Equal(t, types.SideTypeBuy, o.Side)
	}

	closedOrders, trades = engine.SellToPrice(fixedpoint.NewFromFloat(7000.0))
	assert.Len(t, closedOrders, 4)
	assert.Len(t, trades, 4)

	closedOrders, trades = engine.BuyToPrice(fixedpoint.NewFromFloat(8900.0))
	assert.Len(t, closedOrders, 0)
	assert.Len(t, trades, 0)

	closedOrders, trades = engine.BuyToPrice(fixedpoint.NewFromFloat(9000.0))
	assert.Len(t, closedOrders, 1)
	assert.Len(t, trades, 1)
	for _, o := range closedOrders {
		assert.Equal(t, types.SideTypeSell, o.Side)
	}
	for _, trade := range trades {
		assert.Equal(t, types.SideTypeSell, trade.Side)
	}

	closedOrders, trades = engine.BuyToPrice(fixedpoint.NewFromFloat(9500.0))
	assert.Len(t, closedOrders, 4)
	assert.Len(t, trades, 4)
}

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

func TestSimplePriceMatching_orderUpdate(t *testing.T) {
	account := &types.Account{
		MakerFeeRate: fixedpoint.NewFromFloat(0.075 * 0.01),
		TakerFeeRate: fixedpoint.NewFromFloat(0.075 * 0.01),
	}
	account.UpdateBalances(types.BalanceMap{
		"USDT": {Currency: "USDT", Available: fixedpoint.NewFromFloat(10000.0)},
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

	t1 := time.Date(2021, 7, 1, 0, 0, 0, 0, time.UTC)
	engine := &SimplePriceMatching{
		Account:     account,
		Market:      market,
		CurrentTime: t1,
	}

	orderUpdateCnt := 0
	orderUpdateNewStatusCnt := 0
	orderUpdateFilledStatusCnt := 0
	var lastOrder types.Order
	engine.OnOrderUpdate(func(order types.Order) {
		lastOrder = order

		orderUpdateCnt++
		switch order.Status {
		case types.OrderStatusNew:
			orderUpdateNewStatusCnt++

		case types.OrderStatusFilled:
			orderUpdateFilledStatusCnt++

		}
	})

	_, _, err := engine.PlaceOrder(newLimitOrder("BTCUSDT", types.SideTypeBuy, 24000.0, 0.1))
	assert.NoError(t, err)
	assert.Equal(t, 1, orderUpdateCnt)             // should got new status
	assert.Equal(t, 1, orderUpdateNewStatusCnt)    // should got new status
	assert.Equal(t, 0, orderUpdateFilledStatusCnt) // should got new status
	assert.Equal(t, types.OrderStatusNew, lastOrder.Status)
	assert.Equal(t, fixedpoint.NewFromFloat(0.0), lastOrder.ExecutedQuantity)

	t2 := t1.Add(time.Minute)

	// should match 25000, 24000
	k := newKLine("BTCUSDT", types.Interval1m, t2, 26000, 27000, 23000, 25000)
	engine.processKLine(k)

	assert.Equal(t, 2, orderUpdateCnt)             // should got new and filled
	assert.Equal(t, 1, orderUpdateNewStatusCnt)    // should got new status
	assert.Equal(t, 1, orderUpdateFilledStatusCnt) // should got new status
	assert.Equal(t, types.OrderStatusFilled, lastOrder.Status)
	assert.Equal(t, "0.1", lastOrder.ExecutedQuantity.String())
	assert.Equal(t, lastOrder.Quantity.String(), lastOrder.ExecutedQuantity.String())
}

func TestSimplePriceMatching_processKLine(t *testing.T) {
	account := &types.Account{
		MakerFeeRate: fixedpoint.NewFromFloat(0.075 * 0.01),
		TakerFeeRate: fixedpoint.NewFromFloat(0.075 * 0.01),
	}
	account.UpdateBalances(types.BalanceMap{
		"USDT": {Currency: "USDT", Available: fixedpoint.NewFromFloat(10000.0)},
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

	t1 := time.Date(2021, 7, 1, 0, 0, 0, 0, time.UTC)
	engine := &SimplePriceMatching{
		Account:     account,
		Market:      market,
		CurrentTime: t1,
	}

	for i := 0; i <= 5; i++ {
		var p = 20000.0 + float64(i)*1000.0
		_, _, err := engine.PlaceOrder(newLimitOrder("BTCUSDT", types.SideTypeBuy, p, 0.001))
		assert.NoError(t, err)
	}

	t2 := t1.Add(time.Minute)

	// should match 25000, 24000
	k := newKLine("BTCUSDT", types.Interval1m, t2, 26000, 27000, 23000, 25000)
	assert.Equal(t, t2.Add(time.Minute-time.Millisecond), k.EndTime.Time())

	engine.processKLine(k)
	assert.Equal(t, 3, len(engine.bidOrders))
	assert.Len(t, engine.bidOrders, 3)
	assert.Equal(t, 3, len(engine.closedOrders))

	for _, o := range engine.closedOrders {
		assert.Equal(t, k.EndTime.Time(), o.UpdateTime.Time())
	}
}

func newKLine(symbol string, interval types.Interval, startTime time.Time, o, h, l, c float64) types.KLine {
	return types.KLine{
		Symbol:    symbol,
		StartTime: types.Time(startTime),
		EndTime:   types.Time(startTime.Add(interval.Duration() - time.Millisecond)),
		Interval:  interval,
		Open:      fixedpoint.NewFromFloat(o),
		High:      fixedpoint.NewFromFloat(h),
		Low:       fixedpoint.NewFromFloat(l),
		Close:     fixedpoint.NewFromFloat(c),
		Closed:    true,
	}
}

func TestSimplePriceMatching_PlaceLimitOrder(t *testing.T) {
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
		Account: account,
		Market:  market,
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

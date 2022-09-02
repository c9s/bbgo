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
		StepSize:        fixedpoint.MustNewFromString("0.00001"),
		TickSize:        fixedpoint.MustNewFromString("0.01"),
	}

	t1 := time.Date(2021, 7, 1, 0, 0, 0, 0, time.UTC)
	engine := &SimplePriceMatching{
		account:      account,
		Market:       market,
		currentTime:  t1,
		closedOrders: make(map[uint64]types.Order),
		lastPrice:    fixedpoint.NewFromFloat(25000),
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

	// maker order
	_, _, err := engine.PlaceOrder(newLimitOrder("BTCUSDT", types.SideTypeBuy, 24000.0, 0.1))
	assert.NoError(t, err)
	assert.Equal(t, 1, orderUpdateCnt)             // should get new status
	assert.Equal(t, 1, orderUpdateNewStatusCnt)    // should get new status
	assert.Equal(t, 0, orderUpdateFilledStatusCnt) // should get new status
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

func TestSimplePriceMatching_CancelOrder(t *testing.T) {
	account := getTestAccount()
	market := getTestMarket()
	t1 := time.Date(2021, 7, 1, 0, 0, 0, 0, time.UTC)
	engine := &SimplePriceMatching{
		account:      account,
		Market:       market,
		currentTime:  t1,
		closedOrders: make(map[uint64]types.Order),
		lastPrice:    fixedpoint.NewFromFloat(30000.0),
	}

	createdOrder1, trade1, err := engine.PlaceOrder(newLimitOrder("BTCUSDT", types.SideTypeBuy, 20000.0, 0.1))
	assert.NoError(t, err)
	assert.Nil(t, trade1)
	assert.Len(t, engine.bidOrders, 1)
	assert.Len(t, engine.askOrders, 0)

	createdOrder2, trade2, err := engine.PlaceOrder(newLimitOrder("BTCUSDT", types.SideTypeSell, 40000.0, 0.1))
	assert.NoError(t, err)
	assert.Nil(t, trade2)
	assert.Len(t, engine.bidOrders, 1)
	assert.Len(t, engine.askOrders, 1)

	if assert.NotNil(t, createdOrder1) {
		retOrder, err := engine.CancelOrder(*createdOrder1)
		assert.NoError(t, err)
		assert.NotNil(t, retOrder)
		assert.Len(t, engine.bidOrders, 0)
		assert.Len(t, engine.askOrders, 1)
	}

	if assert.NotNil(t, createdOrder2) {
		retOrder, err := engine.CancelOrder(*createdOrder2)
		assert.NoError(t, err)
		assert.NotNil(t, retOrder)
		assert.Len(t, engine.bidOrders, 0)
		assert.Len(t, engine.askOrders, 0)
	}
}

func TestSimplePriceMatching_processKLine(t *testing.T) {
	account := getTestAccount()
	market := getTestMarket()

	t1 := time.Date(2021, 7, 1, 0, 0, 0, 0, time.UTC)
	engine := &SimplePriceMatching{
		account:      account,
		Market:       market,
		currentTime:  t1,
		closedOrders: make(map[uint64]types.Order),
		lastPrice:    fixedpoint.NewFromFloat(30000.0),
	}

	for i := 0; i <= 5; i++ {
		var p = 20000.0 + float64(i)*1000.0
		_, _, err := engine.PlaceOrder(newLimitOrder("BTCUSDT", types.SideTypeBuy, p, 0.001))
		assert.NoError(t, err)
	}

	t2 := t1.Add(time.Minute)

	// should match 25000, 24000
	k := newKLine("BTCUSDT", types.Interval1m, t2, 30000, 27000, 23000, 25000)
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

// getTestMarket returns the BTCUSDT market information
// for tests, we always use BTCUSDT
func getTestMarket() types.Market {
	market := types.Market{
		Symbol:          "BTCUSDT",
		PricePrecision:  8,
		VolumePrecision: 8,
		QuoteCurrency:   "USDT",
		BaseCurrency:    "BTC",
		MinNotional:     fixedpoint.MustNewFromString("0.001"),
		MinAmount:       fixedpoint.MustNewFromString("10.0"),
		MinQuantity:     fixedpoint.MustNewFromString("0.001"),
		StepSize:        fixedpoint.MustNewFromString("0.00001"),
		TickSize:        fixedpoint.MustNewFromString("0.01"),
	}
	return market
}

func getTestAccount() *types.Account {
	account := &types.Account{
		MakerFeeRate: fixedpoint.NewFromFloat(0.075 * 0.01),
		TakerFeeRate: fixedpoint.NewFromFloat(0.075 * 0.01),
	}
	account.UpdateBalances(types.BalanceMap{
		"USDT": {Currency: "USDT", Available: fixedpoint.NewFromFloat(1000000.0)},
		"BTC":  {Currency: "BTC", Available: fixedpoint.NewFromFloat(100.0)},
	})
	return account
}

func TestSimplePriceMatching_LimitBuyTakerOrder(t *testing.T) {
	account := getTestAccount()
	market := getTestMarket()
	engine := &SimplePriceMatching{
		account:      account,
		Market:       market,
		closedOrders: make(map[uint64]types.Order),
		lastPrice:    fixedpoint.NewFromFloat(19000.0),
	}

	takerOrder := types.SubmitOrder{
		Symbol:      market.Symbol,
		Side:        types.SideTypeBuy,
		Type:        types.OrderTypeLimit,
		Quantity:    fixedpoint.NewFromFloat(0.1),
		Price:       fixedpoint.NewFromFloat(20000.0),
		TimeInForce: types.TimeInForceGTC,
	}
	createdOrder, trade, err := engine.PlaceOrder(takerOrder)
	assert.NoError(t, err)
	t.Logf("created order: %+v", createdOrder)
	t.Logf("executed trade: %+v", trade)

	assert.Equal(t, "19000", trade.Price.String())
	assert.Equal(t, "19000", createdOrder.AveragePrice.String())
	assert.Equal(t, "20000", createdOrder.Price.String())

	usdt, ok := account.Balance("USDT")
	assert.True(t, ok)
	assert.True(t, usdt.Locked.IsZero())

	btc, ok := account.Balance("BTC")
	assert.True(t, ok)
	assert.True(t, btc.Locked.IsZero())
	assert.Equal(t, fixedpoint.NewFromFloat(100.0).Add(createdOrder.Quantity).String(), btc.Available.String())

	usedQuoteAmount := createdOrder.AveragePrice.Mul(createdOrder.Quantity)
	assert.Equal(t, "USDT", trade.FeeCurrency)
	assert.Equal(t, usdt.Available.String(), fixedpoint.NewFromFloat(1000000.0).Sub(usedQuoteAmount).Sub(trade.Fee).String())
}

func TestSimplePriceMatching_StopLimitOrderBuy(t *testing.T) {
	account := getTestAccount()
	market := getTestMarket()
	engine := &SimplePriceMatching{
		account:      account,
		Market:       market,
		closedOrders: make(map[uint64]types.Order),
		lastPrice:    fixedpoint.NewFromFloat(19000.0),
	}

	stopBuyOrder := types.SubmitOrder{
		Symbol:      market.Symbol,
		Side:        types.SideTypeBuy,
		Type:        types.OrderTypeStopLimit,
		Quantity:    fixedpoint.NewFromFloat(0.1),
		Price:       fixedpoint.NewFromFloat(22000.0),
		StopPrice:   fixedpoint.NewFromFloat(21000.0),
		TimeInForce: types.TimeInForceGTC,
	}
	createdOrder, trade, err := engine.PlaceOrder(stopBuyOrder)
	assert.NoError(t, err)
	assert.Nil(t, trade, "place stop order should not trigger the stop buy")
	assert.NotNil(t, createdOrder, "place stop order should not trigger the stop buy")

	// place some limit orders, so we ensure that the remaining orders are not removed.
	_, _, err = engine.PlaceOrder(newLimitOrder(market.Symbol, types.SideTypeBuy, 18000, 0.01))
	assert.NoError(t, err)
	_, _, err = engine.PlaceOrder(newLimitOrder(market.Symbol, types.SideTypeSell, 32000, 0.01))
	assert.NoError(t, err)
	assert.Equal(t, 2, len(engine.bidOrders))
	assert.Equal(t, 1, len(engine.askOrders))

	closedOrders, trades := engine.buyToPrice(fixedpoint.NewFromFloat(20000.0))
	assert.Len(t, closedOrders, 0, "price change far from the price should not trigger the stop buy")
	assert.Len(t, trades, 0, "price change far from the price should not trigger the stop buy")
	assert.Equal(t, 2, len(engine.bidOrders), "bid orders should be the same")
	assert.Equal(t, 1, len(engine.askOrders), "ask orders should be the same")

	closedOrders, trades = engine.buyToPrice(fixedpoint.NewFromFloat(21001.0))
	assert.Len(t, closedOrders, 1, "should trigger the stop buy order")
	assert.Len(t, trades, 1, "should have stop order trade executed")

	assert.Equal(t, types.OrderStatusFilled, closedOrders[0].Status)
	assert.Equal(t, types.OrderTypeLimit, closedOrders[0].Type)
	assert.Equal(t, "21001", trades[0].Price.String())
	assert.Equal(t, "22000", closedOrders[0].Price.String(), "order.Price should not be adjusted")

	assert.Equal(t, fixedpoint.NewFromFloat(21001.0).String(), engine.lastPrice.String())

	stopOrder2 := types.SubmitOrder{
		Symbol:      market.Symbol,
		Side:        types.SideTypeBuy,
		Type:        types.OrderTypeStopLimit,
		Quantity:    fixedpoint.NewFromFloat(0.1),
		Price:       fixedpoint.NewFromFloat(22000.0),
		StopPrice:   fixedpoint.NewFromFloat(21000.0),
		TimeInForce: types.TimeInForceGTC,
	}
	createdOrder, trade, err = engine.PlaceOrder(stopOrder2)
	assert.NoError(t, err)
	assert.Nil(t, trade, "place stop order should not trigger the stop buy")
	assert.NotNil(t, createdOrder, "place stop order should not trigger the stop buy")
	assert.Len(t, engine.bidOrders, 2)

	closedOrders, trades = engine.sellToPrice(fixedpoint.NewFromFloat(20500.0))
	assert.Len(t, closedOrders, 1, "should trigger the stop buy order")
	assert.Len(t, trades, 1, "should have stop order trade executed")
	assert.Len(t, engine.bidOrders, 1, "should left one bid order")
}

func TestSimplePriceMatching_StopLimitOrderSell(t *testing.T) {
	account := getTestAccount()
	market := getTestMarket()
	engine := &SimplePriceMatching{
		account:      account,
		Market:       market,
		closedOrders: make(map[uint64]types.Order),
		lastPrice:    fixedpoint.NewFromFloat(22000.0),
	}

	stopSellOrder := types.SubmitOrder{
		Symbol:      market.Symbol,
		Side:        types.SideTypeSell,
		Type:        types.OrderTypeStopLimit,
		Quantity:    fixedpoint.NewFromFloat(0.1),
		Price:       fixedpoint.NewFromFloat(20000.0),
		StopPrice:   fixedpoint.NewFromFloat(21000.0),
		TimeInForce: types.TimeInForceGTC,
	}
	createdOrder, trade, err := engine.PlaceOrder(stopSellOrder)
	assert.NoError(t, err)
	assert.Nil(t, trade, "place stop order should not trigger the stop sell")
	assert.NotNil(t, createdOrder, "place stop order should not trigger the stop sell")

	// place some limit orders, so we ensure that the remaining orders are not removed.
	_, _, err = engine.PlaceOrder(newLimitOrder(market.Symbol, types.SideTypeBuy, 18000, 0.01))
	assert.NoError(t, err)
	_, _, err = engine.PlaceOrder(newLimitOrder(market.Symbol, types.SideTypeSell, 32000, 0.01))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(engine.bidOrders))
	assert.Equal(t, 2, len(engine.askOrders))

	closedOrders, trades := engine.sellToPrice(fixedpoint.NewFromFloat(21500.0))
	assert.Len(t, closedOrders, 0, "price change far from the price should not trigger the stop buy")
	assert.Len(t, trades, 0, "price change far from the price should not trigger the stop buy")
	assert.Equal(t, 1, len(engine.bidOrders))
	assert.Equal(t, 2, len(engine.askOrders))

	closedOrders, trades = engine.sellToPrice(fixedpoint.NewFromFloat(20990.0))
	assert.Len(t, closedOrders, 1, "should trigger the stop sell order")
	assert.Len(t, trades, 1, "should have stop order trade executed")
	assert.Equal(t, 1, len(engine.bidOrders))
	assert.Equal(t, 1, len(engine.askOrders))

	assert.Equal(t, types.OrderStatusFilled, closedOrders[0].Status)
	assert.Equal(t, types.OrderTypeLimit, closedOrders[0].Type)
	assert.Equal(t, "20000", closedOrders[0].Price.String(), "limit order price should not be changed")
	assert.Equal(t, "20990", trades[0].Price.String())
	assert.Equal(t, "20990", engine.lastPrice.String())

	// place a stop limit sell order with a higher price than the current price
	stopOrder2 := types.SubmitOrder{
		Symbol:      market.Symbol,
		Side:        types.SideTypeSell,
		Type:        types.OrderTypeStopLimit,
		Quantity:    fixedpoint.NewFromFloat(0.1),
		Price:       fixedpoint.NewFromFloat(20000.0),
		StopPrice:   fixedpoint.NewFromFloat(21000.0),
		TimeInForce: types.TimeInForceGTC,
	}

	createdOrder, trade, err = engine.PlaceOrder(stopOrder2)
	assert.NoError(t, err)
	assert.Nil(t, trade, "place stop order should not trigger the stop sell")
	assert.NotNil(t, createdOrder, "place stop order should not trigger the stop sell")

	closedOrders, trades = engine.buyToPrice(fixedpoint.NewFromFloat(21000.0))
	if assert.Len(t, closedOrders, 1, "should trigger the stop sell order") {
		assert.Len(t, trades, 1, "should have stop order trade executed")
		assert.Equal(t, types.SideTypeSell, closedOrders[0].Side)
		assert.Equal(t, types.OrderStatusFilled, closedOrders[0].Status)
		assert.Equal(t, types.OrderTypeLimit, closedOrders[0].Type)
		assert.Equal(t, "21000", trades[0].Price.String(), "trade price should be the kline price not the order price")
		assert.Equal(t, "21000", engine.lastPrice.String(), "engine last price should be updated correctly")
	}
}

func TestSimplePriceMatching_StopMarketOrderSell(t *testing.T) {
	account := getTestAccount()
	market := getTestMarket()
	engine := &SimplePriceMatching{
		account:      account,
		Market:       market,
		closedOrders: make(map[uint64]types.Order),
		lastPrice:    fixedpoint.NewFromFloat(22000.0),
	}

	stopOrder := types.SubmitOrder{
		Symbol:      market.Symbol,
		Side:        types.SideTypeSell,
		Type:        types.OrderTypeStopMarket,
		Quantity:    fixedpoint.NewFromFloat(0.1),
		Price:       fixedpoint.NewFromFloat(20000.0),
		StopPrice:   fixedpoint.NewFromFloat(21000.0),
		TimeInForce: types.TimeInForceGTC,
	}
	createdOrder, trade, err := engine.PlaceOrder(stopOrder)
	assert.NoError(t, err)
	assert.Nil(t, trade, "place stop order should not trigger the stop sell")
	assert.NotNil(t, createdOrder, "place stop order should not trigger the stop sell")

	closedOrders, trades := engine.sellToPrice(fixedpoint.NewFromFloat(21500.0))
	assert.Len(t, closedOrders, 0, "price change far from the price should not trigger the stop buy")
	assert.Len(t, trades, 0, "price change far from the price should not trigger the stop buy")

	closedOrders, trades = engine.sellToPrice(fixedpoint.NewFromFloat(20990.0))
	assert.Len(t, closedOrders, 1, "should trigger the stop sell order")
	assert.Len(t, trades, 1, "should have stop order trade executed")

	assert.Equal(t, types.OrderStatusFilled, closedOrders[0].Status)
	assert.Equal(t, types.OrderTypeMarket, closedOrders[0].Type)
	assert.Equal(t, fixedpoint.NewFromFloat(20990.0), trades[0].Price, "trade price should be adjusted to the last price")
}

func TestSimplePriceMatching_PlaceLimitOrder(t *testing.T) {
	account := getTestAccount()
	market := getTestMarket()
	engine := &SimplePriceMatching{
		account:      account,
		Market:       market,
		closedOrders: make(map[uint64]types.Order),
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

	closedOrders, trades := engine.sellToPrice(fixedpoint.NewFromFloat(8100.0))
	assert.Len(t, closedOrders, 0)
	assert.Len(t, trades, 0)

	closedOrders, trades = engine.sellToPrice(fixedpoint.NewFromFloat(8000.0))
	assert.Len(t, closedOrders, 1)
	assert.Len(t, trades, 1)
	for _, trade := range trades {
		assert.True(t, trade.IsBuyer)
	}

	for _, o := range closedOrders {
		assert.Equal(t, types.SideTypeBuy, o.Side)
	}

	closedOrders, trades = engine.sellToPrice(fixedpoint.NewFromFloat(7000.0))
	assert.Len(t, closedOrders, 4)
	assert.Len(t, trades, 4)

	closedOrders, trades = engine.buyToPrice(fixedpoint.NewFromFloat(8900.0))
	assert.Len(t, closedOrders, 0)
	assert.Len(t, trades, 0)

	closedOrders, trades = engine.buyToPrice(fixedpoint.NewFromFloat(9000.0))
	assert.Len(t, closedOrders, 1)
	assert.Len(t, trades, 1)
	for _, o := range closedOrders {
		assert.Equal(t, types.SideTypeSell, o.Side)
	}
	for _, trade := range trades {
		assert.Equal(t, types.SideTypeSell, trade.Side)
	}

	closedOrders, trades = engine.buyToPrice(fixedpoint.NewFromFloat(9500.0))
	assert.Len(t, closedOrders, 4)
	assert.Len(t, trades, 4)
}

func TestSimplePriceMatching_LimitTakerOrder(t *testing.T) {
	account := getTestAccount()
	market := getTestMarket()
	engine := &SimplePriceMatching{
		account:      account,
		Market:       market,
		closedOrders: make(map[uint64]types.Order),
		lastPrice:    fixedpoint.NewFromFloat(20000.0),
	}

	closedOrder, trade, err := engine.PlaceOrder(newLimitOrder("BTCUSDT", types.SideTypeBuy, 21000.0, 1.0))
	assert.NoError(t, err)
	if assert.NotNil(t, closedOrder) {
		if assert.NotNil(t, trade) {
			assert.Equal(t, "20000", trade.Price.String())
			assert.False(t, trade.IsMaker, "should be taker")
		}
	}

	closedOrder, trade, err = engine.PlaceOrder(newLimitOrder("BTCUSDT", types.SideTypeSell, 19000.0, 1.0))
	assert.NoError(t, err)
	if assert.NotNil(t, closedOrder) {
		assert.Equal(t, "19000", closedOrder.Price.String())
		if assert.NotNil(t, trade) {
			assert.Equal(t, "20000", trade.Price.String())
			assert.False(t, trade.IsMaker, "should be taker")
		}
	}
}

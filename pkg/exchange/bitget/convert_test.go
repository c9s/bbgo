package bitget

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	v2 "github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi/v2"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_toGlobalBalance(t *testing.T) {
	// sample:
	// {
	//        "coinId":"10012",
	//        "coinName":"usdt",
	//        "available":"0",
	//        "frozen":"0",
	//        "lock":"0",
	//        "uTime":"1622697148"
	//    }
	asset := v2.AccountAsset{
		Coin:           "USDT",
		Available:      fixedpoint.NewFromFloat(1.2),
		Frozen:         fixedpoint.NewFromFloat(0.5),
		Locked:         fixedpoint.NewFromFloat(0.5),
		LimitAvailable: fixedpoint.Zero,
		UpdatedTime:    types.NewMillisecondTimestampFromInt(1622697148),
	}

	assert.Equal(t, types.Balance{
		Currency:          "USDT",
		Available:         fixedpoint.NewFromFloat(1.2),
		Locked:            fixedpoint.NewFromFloat(1), // frozen + lock
		Borrowed:          fixedpoint.Zero,
		Interest:          fixedpoint.Zero,
		NetAsset:          fixedpoint.Zero,
		MaxWithdrawAmount: fixedpoint.Zero,
	}, toGlobalBalance(asset))
}

func Test_toGlobalMarket(t *testing.T) {
	// sample:
	// {
	//   "symbol":"BTCUSDT",
	//   "baseCoin":"BTC",
	//   "quoteCoin":"USDT",
	//   "minTradeAmount":"0",
	//   "maxTradeAmount":"10000000000",
	//   "takerFeeRate":"0.002",
	//   "makerFeeRate":"0.002",
	//   "pricePrecision":"2",
	//   "quantityPrecision":"4",
	//   "quotePrecision":"6",
	//   "status":"online",
	//   "minTradeUSDT":"5",
	//   "buyLimitPriceRatio":"0.05",
	//   "sellLimitPriceRatio":"0.05"
	// }
	inst := v2.Symbol{
		Symbol:              "BTCUSDT",
		BaseCoin:            "BTC",
		QuoteCoin:           "USDT",
		MinTradeAmount:      fixedpoint.NewFromFloat(0),
		MaxTradeAmount:      fixedpoint.NewFromFloat(10000000000),
		TakerFeeRate:        fixedpoint.NewFromFloat(0.002),
		MakerFeeRate:        fixedpoint.NewFromFloat(0.002),
		PricePrecision:      fixedpoint.NewFromFloat(2),
		QuantityPrecision:   fixedpoint.NewFromFloat(4),
		QuotePrecision:      fixedpoint.NewFromFloat(6),
		MinTradeUSDT:        fixedpoint.NewFromFloat(5),
		Status:              v2.SymbolStatusOnline,
		BuyLimitPriceRatio:  fixedpoint.NewFromFloat(0.05),
		SellLimitPriceRatio: fixedpoint.NewFromFloat(0.05),
	}

	exp := types.Market{
		Exchange:        types.ExchangeBitget,
		Symbol:          inst.Symbol,
		LocalSymbol:     inst.Symbol,
		PricePrecision:  2,
		VolumePrecision: 4,
		QuoteCurrency:   inst.QuoteCoin,
		BaseCurrency:    inst.BaseCoin,
		MinNotional:     inst.MinTradeUSDT,
		MinAmount:       inst.MinTradeUSDT,
		MinQuantity:     inst.MinTradeAmount,
		MaxQuantity:     inst.MaxTradeAmount,
		StepSize:        fixedpoint.NewFromFloat(0.0001),
		MinPrice:        fixedpoint.Zero,
		MaxPrice:        fixedpoint.Zero,
		TickSize:        fixedpoint.NewFromFloat(0.01),
	}

	assert.Equal(t, toGlobalMarket(inst), exp)
}

func Test_toGlobalTicker(t *testing.T) {
	// sample:
	// {
	//   "open":"36465.96",
	//   "symbol":"BTCUSDT",
	//   "high24h":"37040.25",
	//   "low24h":"36202.65",
	//   "lastPr":"36684.42",
	//   "quoteVolume":"311893591.2805",
	//   "baseVolume":"8507.3684",
	//   "usdtVolume":"311893591.280427",
	//   "ts":"1699947106122",
	//   "bidPr":"36684.49",
	//   "askPr":"36684.51",
	//   "bidSz":"0.3812",
	//   "askSz":"0.0133",
	//   "openUtc":"36465.96",
	//   "changeUtc24h":"0.00599",
	//   "change24h":"-0.00426"
	// }
	ticker := v2.Ticker{
		Symbol:       "BTCUSDT",
		High24H:      fixedpoint.NewFromFloat(24175.65),
		Low24H:       fixedpoint.NewFromFloat(23677.75),
		LastPr:       fixedpoint.NewFromFloat(24014.11),
		QuoteVolume:  fixedpoint.NewFromFloat(177689342.3025),
		BaseVolume:   fixedpoint.NewFromFloat(7421.5009),
		UsdtVolume:   fixedpoint.NewFromFloat(177689342.302407),
		Ts:           types.NewMillisecondTimestampFromInt(1660704288118),
		BidPr:        fixedpoint.NewFromFloat(24013.94),
		AskPr:        fixedpoint.NewFromFloat(24014.06),
		BidSz:        fixedpoint.NewFromFloat(0.0663),
		AskSz:        fixedpoint.NewFromFloat(0.0119),
		OpenUtc:      fixedpoint.NewFromFloat(23856.72),
		ChangeUtc24H: fixedpoint.NewFromFloat(0.00301),
		Change24H:    fixedpoint.NewFromFloat(0.00069),
		Open:         fixedpoint.NewFromFloat(23856.72),
	}

	assert.Equal(t, types.Ticker{
		Time:   types.NewMillisecondTimestampFromInt(1660704288118).Time(),
		Volume: fixedpoint.NewFromFloat(7421.5009),
		Last:   fixedpoint.NewFromFloat(24014.11),
		Open:   fixedpoint.NewFromFloat(23856.72),
		High:   fixedpoint.NewFromFloat(24175.65),
		Low:    fixedpoint.NewFromFloat(23677.75),
		Buy:    fixedpoint.NewFromFloat(24013.94),
		Sell:   fixedpoint.NewFromFloat(24014.06),
	}, toGlobalTicker(ticker))
}

func Test_toGlobalSideType(t *testing.T) {
	side, err := toGlobalSideType(v2.SideTypeBuy)
	assert.NoError(t, err)
	assert.Equal(t, types.SideTypeBuy, side)

	side, err = toGlobalSideType(v2.SideTypeSell)
	assert.NoError(t, err)
	assert.Equal(t, types.SideTypeSell, side)

	_, err = toGlobalSideType("xxx")
	assert.ErrorContains(t, err, "xxx")
}

func Test_toGlobalOrderType(t *testing.T) {
	orderType, err := toGlobalOrderType(v2.OrderTypeMarket)
	assert.NoError(t, err)
	assert.Equal(t, types.OrderTypeMarket, orderType)

	orderType, err = toGlobalOrderType(v2.OrderTypeLimit)
	assert.NoError(t, err)
	assert.Equal(t, types.OrderTypeLimit, orderType)

	_, err = toGlobalOrderType("xxx")
	assert.ErrorContains(t, err, "xxx")
}

func Test_toGlobalOrderStatus(t *testing.T) {
	status, err := toGlobalOrderStatus(v2.OrderStatusInit)
	assert.NoError(t, err)
	assert.Equal(t, types.OrderStatusNew, status)

	status, err = toGlobalOrderStatus(v2.OrderStatusNew)
	assert.NoError(t, err)
	assert.Equal(t, types.OrderStatusNew, status)

	status, err = toGlobalOrderStatus(v2.OrderStatusLive)
	assert.NoError(t, err)
	assert.Equal(t, types.OrderStatusNew, status)

	status, err = toGlobalOrderStatus(v2.OrderStatusFilled)
	assert.NoError(t, err)
	assert.Equal(t, types.OrderStatusFilled, status)

	status, err = toGlobalOrderStatus(v2.OrderStatusPartialFilled)
	assert.NoError(t, err)
	assert.Equal(t, types.OrderStatusPartiallyFilled, status)

	status, err = toGlobalOrderStatus(v2.OrderStatusCancelled)
	assert.NoError(t, err)
	assert.Equal(t, types.OrderStatusCanceled, status)

	_, err = toGlobalOrderStatus("xxx")
	assert.ErrorContains(t, err, "xxx")
}

func Test_unfilledOrderToGlobalOrder(t *testing.T) {
	var (
		assert        = assert.New(t)
		orderId       = 1105087175647989764
		unfilledOrder = v2.UnfilledOrder{
			Symbol:           "BTCUSDT",
			OrderId:          types.StrInt64(orderId),
			ClientOrderId:    "74b86af3-6098-479c-acac-bfb074c067f3",
			PriceAvg:         fixedpoint.NewFromFloat(1.2),
			Size:             fixedpoint.NewFromFloat(5),
			OrderType:        v2.OrderTypeLimit,
			Side:             v2.SideTypeBuy,
			Status:           v2.OrderStatusLive,
			BasePrice:        fixedpoint.NewFromFloat(0),
			BaseVolume:       fixedpoint.NewFromFloat(0),
			QuoteVolume:      fixedpoint.NewFromFloat(0),
			EnterPointSource: "API",
			OrderSource:      "normal",
			CreatedTime:      types.NewMillisecondTimestampFromInt(1660704288118),
			UpdatedTime:      types.NewMillisecondTimestampFromInt(1660704288118),
		}
	)

	t.Run("succeeds with limit order", func(t *testing.T) {
		order, err := unfilledOrderToGlobalOrder(unfilledOrder)
		assert.NoError(err)
		assert.Equal(&types.Order{
			SubmitOrder: types.SubmitOrder{
				ClientOrderID: "74b86af3-6098-479c-acac-bfb074c067f3",
				Symbol:        "BTCUSDT",
				Side:          types.SideTypeBuy,
				Type:          types.OrderTypeLimit,
				Quantity:      fixedpoint.NewFromFloat(5),
				Price:         fixedpoint.NewFromFloat(1.2),
				TimeInForce:   types.TimeInForceGTC,
			},
			Exchange:         types.ExchangeBitget,
			OrderID:          uint64(orderId),
			UUID:             strconv.FormatInt(int64(orderId), 10),
			Status:           types.OrderStatusNew,
			ExecutedQuantity: fixedpoint.NewFromFloat(0),
			IsWorking:        true,
			CreationTime:     types.Time(types.NewMillisecondTimestampFromInt(1660704288118).Time()),
			UpdateTime:       types.Time(types.NewMillisecondTimestampFromInt(1660704288118).Time()),
		}, order)
	})

	t.Run("succeeds with market buy order", func(t *testing.T) {
		unfilledOrder2 := unfilledOrder
		unfilledOrder2.OrderType = v2.OrderTypeMarket
		unfilledOrder2.Side = v2.SideTypeBuy
		unfilledOrder2.Size = unfilledOrder2.PriceAvg.Mul(unfilledOrder2.Size)
		unfilledOrder2.PriceAvg = fixedpoint.Zero
		unfilledOrder2.Status = v2.OrderStatusNew

		order, err := unfilledOrderToGlobalOrder(unfilledOrder2)
		assert.NoError(err)
		assert.Equal(&types.Order{
			SubmitOrder: types.SubmitOrder{
				ClientOrderID: "74b86af3-6098-479c-acac-bfb074c067f3",
				Symbol:        "BTCUSDT",
				Side:          types.SideTypeBuy,
				Type:          types.OrderTypeMarket,
				Quantity:      fixedpoint.Zero,
				Price:         fixedpoint.Zero,
				TimeInForce:   types.TimeInForceGTC,
			},
			Exchange:         types.ExchangeBitget,
			OrderID:          uint64(orderId),
			UUID:             strconv.FormatInt(int64(orderId), 10),
			Status:           types.OrderStatusNew,
			ExecutedQuantity: fixedpoint.Zero,
			IsWorking:        true,
			CreationTime:     types.Time(types.NewMillisecondTimestampFromInt(1660704288118).Time()),
			UpdateTime:       types.Time(types.NewMillisecondTimestampFromInt(1660704288118).Time()),
		}, order)
	})

	t.Run("succeeds with market sell order", func(t *testing.T) {
		unfilledOrder2 := unfilledOrder
		unfilledOrder2.OrderType = v2.OrderTypeMarket
		unfilledOrder2.Side = v2.SideTypeSell
		unfilledOrder2.PriceAvg = fixedpoint.Zero
		unfilledOrder2.Status = v2.OrderStatusNew

		order, err := unfilledOrderToGlobalOrder(unfilledOrder2)
		assert.NoError(err)
		assert.Equal(&types.Order{
			SubmitOrder: types.SubmitOrder{
				ClientOrderID: "74b86af3-6098-479c-acac-bfb074c067f3",
				Symbol:        "BTCUSDT",
				Side:          types.SideTypeSell,
				Type:          types.OrderTypeMarket,
				Quantity:      fixedpoint.NewFromInt(5),
				Price:         fixedpoint.Zero,
				TimeInForce:   types.TimeInForceGTC,
			},
			Exchange:         types.ExchangeBitget,
			OrderID:          uint64(orderId),
			UUID:             strconv.FormatInt(int64(orderId), 10),
			Status:           types.OrderStatusNew,
			ExecutedQuantity: fixedpoint.Zero,
			IsWorking:        true,
			CreationTime:     types.Time(types.NewMillisecondTimestampFromInt(1660704288118).Time()),
			UpdateTime:       types.Time(types.NewMillisecondTimestampFromInt(1660704288118).Time()),
		}, order)
	})

	t.Run("failed to convert side", func(t *testing.T) {
		newOrder := unfilledOrder
		newOrder.Side = "xxx"

		_, err := unfilledOrderToGlobalOrder(newOrder)
		assert.ErrorContains(err, "xxx")
	})

	t.Run("failed to convert oder type", func(t *testing.T) {
		newOrder := unfilledOrder
		newOrder.OrderType = "xxx"

		_, err := unfilledOrderToGlobalOrder(newOrder)
		assert.ErrorContains(err, "xxx")
	})

	t.Run("failed to convert oder status", func(t *testing.T) {
		newOrder := unfilledOrder
		newOrder.Status = "xxx"

		_, err := unfilledOrderToGlobalOrder(newOrder)
		assert.ErrorContains(err, "xxx")
	})
}

func Test_toGlobalOrder(t *testing.T) {
	var (
		assert        = assert.New(t)
		orderId       = 1105087175647989764
		unfilledOrder = v2.OrderDetail{
			UserId:           123456,
			Symbol:           "BTCUSDT",
			OrderId:          types.StrInt64(orderId),
			ClientOrderId:    "74b86af3-6098-479c-acac-bfb074c067f3",
			Price:            fixedpoint.NewFromFloat(1.2),
			Size:             fixedpoint.NewFromFloat(5),
			OrderType:        v2.OrderTypeLimit,
			Side:             v2.SideTypeBuy,
			Status:           v2.OrderStatusFilled,
			PriceAvg:         fixedpoint.NewFromFloat(1.4),
			BaseVolume:       fixedpoint.NewFromFloat(5),
			QuoteVolume:      fixedpoint.NewFromFloat(7.0005),
			EnterPointSource: "API",
			FeeDetailRaw:     `{\"newFees\":{\"c\":0,\"d\":0,\"deduction\":false,\"r\":-0.0070005,\"t\":-0.0070005,\"totalDeductionFee\":0},\"USDT\":{\"deduction\":false,\"feeCoinCode\":\"USDT\",\"totalDeductionFee\":0,\"totalFee\":-0.007000500000}}`,
			OrderSource:      "normal",
			CreatedTime:      types.NewMillisecondTimestampFromInt(1660704288118),
			UpdatedTime:      types.NewMillisecondTimestampFromInt(1660704288118),
		}

		expOrder = &types.Order{
			SubmitOrder: types.SubmitOrder{
				ClientOrderID: "74b86af3-6098-479c-acac-bfb074c067f3",
				Symbol:        "BTCUSDT",
				Side:          types.SideTypeBuy,
				Type:          types.OrderTypeLimit,
				Quantity:      fixedpoint.NewFromFloat(5),
				Price:         fixedpoint.NewFromFloat(1.2),
				TimeInForce:   types.TimeInForceGTC,
			},
			Exchange:         types.ExchangeBitget,
			OrderID:          uint64(orderId),
			UUID:             strconv.FormatInt(int64(orderId), 10),
			Status:           types.OrderStatusFilled,
			ExecutedQuantity: fixedpoint.NewFromFloat(5),
			IsWorking:        false,
			CreationTime:     types.Time(types.NewMillisecondTimestampFromInt(1660704288118).Time()),
			UpdateTime:       types.Time(types.NewMillisecondTimestampFromInt(1660704288118).Time()),
		}
	)

	t.Run("succeeds with limit buy", func(t *testing.T) {
		order, err := toGlobalOrder(unfilledOrder)
		assert.NoError(err)
		assert.Equal(expOrder, order)
	})

	t.Run("succeeds with limit sell", func(t *testing.T) {
		newUnfilledOrder := unfilledOrder
		newUnfilledOrder.Side = v2.SideTypeSell

		newExpOrder := *expOrder
		newExpOrder.Side = types.SideTypeSell

		order, err := toGlobalOrder(newUnfilledOrder)
		assert.NoError(err)
		assert.Equal(&newExpOrder, order)
	})

	t.Run("succeeds with market sell", func(t *testing.T) {
		newUnfilledOrder := unfilledOrder
		newUnfilledOrder.Side = v2.SideTypeSell
		newUnfilledOrder.OrderType = v2.OrderTypeMarket

		newExpOrder := *expOrder
		newExpOrder.Side = types.SideTypeSell
		newExpOrder.Type = types.OrderTypeMarket
		newExpOrder.Price = newUnfilledOrder.PriceAvg

		order, err := toGlobalOrder(newUnfilledOrder)
		assert.NoError(err)
		assert.Equal(&newExpOrder, order)
	})

	t.Run("succeeds with market buy", func(t *testing.T) {
		newUnfilledOrder := unfilledOrder
		newUnfilledOrder.Side = v2.SideTypeBuy
		newUnfilledOrder.OrderType = v2.OrderTypeMarket

		newExpOrder := *expOrder
		newExpOrder.Side = types.SideTypeBuy
		newExpOrder.Type = types.OrderTypeMarket
		newExpOrder.Price = newUnfilledOrder.PriceAvg
		newExpOrder.Quantity = newUnfilledOrder.BaseVolume

		order, err := toGlobalOrder(newUnfilledOrder)
		assert.NoError(err)
		assert.Equal(&newExpOrder, order)
	})

	t.Run("succeeds with limit buy", func(t *testing.T) {
		order, err := toGlobalOrder(unfilledOrder)
		assert.NoError(err)
		assert.Equal(&types.Order{
			SubmitOrder: types.SubmitOrder{
				ClientOrderID: "74b86af3-6098-479c-acac-bfb074c067f3",
				Symbol:        "BTCUSDT",
				Side:          types.SideTypeBuy,
				Type:          types.OrderTypeLimit,
				Quantity:      fixedpoint.NewFromFloat(5),
				Price:         fixedpoint.NewFromFloat(1.2),
				TimeInForce:   types.TimeInForceGTC,
			},
			Exchange:         types.ExchangeBitget,
			OrderID:          uint64(orderId),
			UUID:             strconv.FormatInt(int64(orderId), 10),
			Status:           types.OrderStatusFilled,
			ExecutedQuantity: fixedpoint.NewFromFloat(5),
			IsWorking:        false,
			CreationTime:     types.Time(types.NewMillisecondTimestampFromInt(1660704288118).Time()),
			UpdateTime:       types.Time(types.NewMillisecondTimestampFromInt(1660704288118).Time()),
		}, order)
	})

	t.Run("failed to convert side", func(t *testing.T) {
		newOrder := unfilledOrder
		newOrder.Side = "xxx"

		_, err := toGlobalOrder(newOrder)
		assert.ErrorContains(err, "xxx")
	})

	t.Run("failed to convert oder type", func(t *testing.T) {
		newOrder := unfilledOrder
		newOrder.OrderType = "xxx"

		_, err := toGlobalOrder(newOrder)
		assert.ErrorContains(err, "xxx")
	})

	t.Run("failed to convert oder status", func(t *testing.T) {
		newOrder := unfilledOrder
		newOrder.Status = "xxx"

		_, err := toGlobalOrder(newOrder)
		assert.ErrorContains(err, "xxx")
	})
}

func Test_processMarketBuyQuantity(t *testing.T) {
	var (
		assert            = assert.New(t)
		filledBaseCoinQty = fixedpoint.NewFromFloat(3.5648)
		filledPrice       = fixedpoint.NewFromFloat(4.99998848)
		priceAvg          = fixedpoint.NewFromFloat(1.4026)
		buyQty            = fixedpoint.NewFromFloat(5)
	)

	t.Run("zero quantity on Init/New/Live/Cancelled", func(t *testing.T) {
		qty, err := processMarketBuyQuantity(filledBaseCoinQty, filledPrice, priceAvg, buyQty, v2.OrderStatusInit)
		assert.NoError(err)
		assert.Equal(fixedpoint.Zero, qty)

		qty, err = processMarketBuyQuantity(filledBaseCoinQty, filledPrice, priceAvg, buyQty, v2.OrderStatusNew)
		assert.NoError(err)
		assert.Equal(fixedpoint.Zero, qty)

		qty, err = processMarketBuyQuantity(filledBaseCoinQty, filledPrice, priceAvg, buyQty, v2.OrderStatusLive)
		assert.NoError(err)
		assert.Equal(fixedpoint.Zero, qty)

		qty, err = processMarketBuyQuantity(filledBaseCoinQty, filledPrice, priceAvg, buyQty, v2.OrderStatusCancelled)
		assert.NoError(err)
		assert.Equal(fixedpoint.Zero, qty)
	})

	t.Run("5 on PartialFilled", func(t *testing.T) {
		priceAvg := fixedpoint.NewFromFloat(2)
		buyQty := fixedpoint.NewFromFloat(10)
		filledPrice := fixedpoint.NewFromFloat(4)
		filledBaseCoinQty := fixedpoint.NewFromFloat(2)

		qty, err := processMarketBuyQuantity(filledBaseCoinQty, filledPrice, priceAvg, buyQty, v2.OrderStatusPartialFilled)
		assert.NoError(err)
		assert.Equal(fixedpoint.NewFromFloat(5), qty)
	})

	t.Run("3.5648 on Filled", func(t *testing.T) {
		qty, err := processMarketBuyQuantity(filledBaseCoinQty, filledPrice, priceAvg, buyQty, v2.OrderStatusFilled)
		assert.NoError(err)
		assert.Equal(fixedpoint.NewFromFloat(3.5648), qty)
	})

	t.Run("unexpected order status", func(t *testing.T) {
		_, err := processMarketBuyQuantity(filledBaseCoinQty, filledPrice, priceAvg, buyQty, "xxx")
		assert.ErrorContains(err, "xxx")
	})
}

func Test_toLocalOrderType(t *testing.T) {
	orderType, err := toLocalOrderType(types.OrderTypeLimit)
	assert.NoError(t, err)
	assert.Equal(t, v2.OrderTypeLimit, orderType)

	orderType, err = toLocalOrderType(types.OrderTypeMarket)
	assert.NoError(t, err)
	assert.Equal(t, v2.OrderTypeMarket, orderType)

	_, err = toLocalOrderType("xxx")
	assert.ErrorContains(t, err, "xxx")
}

func Test_toLocalSide(t *testing.T) {
	orderType, err := toLocalSide(types.SideTypeSell)
	assert.NoError(t, err)
	assert.Equal(t, v2.SideTypeSell, orderType)

	orderType, err = toLocalSide(types.SideTypeBuy)
	assert.NoError(t, err)
	assert.Equal(t, v2.SideTypeBuy, orderType)

	_, err = toLocalOrderType("xxx")
	assert.ErrorContains(t, err, "xxx")
}

func Test_isMaker(t *testing.T) {
	isM, err := isMaker(v2.TradeTaker)
	assert.NoError(t, err)
	assert.False(t, isM)

	isM, err = isMaker(v2.TradeMaker)
	assert.NoError(t, err)
	assert.True(t, isM)

	_, err = isMaker("xxx")
	assert.ErrorContains(t, err, "xxx")
}

func Test_isFeeDiscount(t *testing.T) {
	isDiscount, err := isFeeDiscount(v2.DiscountNo)
	assert.NoError(t, err)
	assert.False(t, isDiscount)

	isDiscount, err = isFeeDiscount(v2.DiscountYes)
	assert.NoError(t, err)
	assert.True(t, isDiscount)

	_, err = isFeeDiscount("xxx")
	assert.ErrorContains(t, err, "xxx")
}

func Test_toGlobalTrade(t *testing.T) {
	// {
	//   "userId":"8672173294",
	//   "symbol":"APEUSDT",
	//   "orderId":"1104337778433757184",
	//   "tradeId":"1104337778504044545",
	//   "orderType":"limit",
	//   "side":"sell",
	//   "priceAvg":"1.4001",
	//   "newSize":"5",
	//   "amount":"7.0005",
	//   "feeDetail":{
	//      "deduction":"no",
	//      "feeCoin":"USDT",
	//      "totalDeductionFee":"",
	//      "totalFee":"-0.0070005"
	//   },
	//   "tradeScope":"taker",
	//   "cTime":"1699020564676",
	//   "uTime":"1699020564687"
	// }
	trade := v2.Trade{
		UserId:    types.StrInt64(8672173294),
		Symbol:    "APEUSDT",
		OrderId:   types.StrInt64(1104337778433757184),
		TradeId:   types.StrInt64(1104337778504044545),
		OrderType: v2.OrderTypeLimit,
		Side:      v2.SideTypeSell,
		PriceAvg:  fixedpoint.NewFromFloat(1.4001),
		Size:      fixedpoint.NewFromFloat(5),
		Amount:    fixedpoint.NewFromFloat(7.0005),
		FeeDetail: v2.TradeFee{
			Deduction:         "no",
			FeeCoin:           "USDT",
			TotalDeductionFee: fixedpoint.Zero,
			TotalFee:          fixedpoint.NewFromFloat(-0.0070005),
		},
		TradeScope:  v2.TradeTaker,
		CreatedTime: types.NewMillisecondTimestampFromInt(1699020564676),
		UpdatedTime: types.NewMillisecondTimestampFromInt(1699020564687),
	}

	res, err := toGlobalTrade(trade)
	assert.NoError(t, err)
	assert.Equal(t, &types.Trade{
		ID:            uint64(1104337778504044545),
		OrderID:       uint64(1104337778433757184),
		Exchange:      types.ExchangeBitget,
		Price:         fixedpoint.NewFromFloat(1.4001),
		Quantity:      fixedpoint.NewFromFloat(5),
		QuoteQuantity: fixedpoint.NewFromFloat(7.0005),
		Symbol:        "APEUSDT",
		Side:          types.SideTypeSell,
		IsBuyer:       false,
		IsMaker:       false,
		Time:          types.Time(types.NewMillisecondTimestampFromInt(1699020564676)),
		Fee:           fixedpoint.NewFromFloat(0.0070005),
		FeeCurrency:   "USDT",
		FeeDiscounted: false,
	}, res)
}

func Test_toGlobalBalanceMap(t *testing.T) {
	assert.Equal(t, types.BalanceMap{
		"BTC": {
			Currency:  "BTC",
			Available: fixedpoint.NewFromFloat(0.5),
			Locked:    fixedpoint.NewFromFloat(0.6 + 0.7),
		},
	}, toGlobalBalanceMap([]Balance{
		{
			Coin:           "BTC",
			Available:      fixedpoint.NewFromFloat(0.5),
			Frozen:         fixedpoint.NewFromFloat(0.6),
			Locked:         fixedpoint.NewFromFloat(0.7),
			LimitAvailable: fixedpoint.Zero,
			UpdatedTime:    types.NewMillisecondTimestampFromInt(1699020564676),
		},
	}))
}

func Test_toGlobalKLines(t *testing.T) {
	symbol := "BTCUSDT"
	interval := types.Interval15m

	resp := v2.KLineResponse{
		/*
			[
				{
					"Ts": "1699816800000",
					"OpenPrice": 29045.3,
					"HighPrice": 29228.56,
					"LowPrice": 29045.3,
					"ClosePrice": 29228.56,
					"Volume": 9.265593,
					"QuoteVolume": 270447.43520753,
					"UsdtVolume": 270447.43520753
				},
				{
					"Ts": "1699816800000",
					"OpenPrice": 29167.33,
					"HighPrice": 29229.08,
					"LowPrice": 29000,
					"ClosePrice": 29045.3,
					"Volume": 9.295508,
					"QuoteVolume": 270816.87513775,
					"UsdtVolume": 270816.87513775
				}
			]
		*/
		{
			Ts:          types.NewMillisecondTimestampFromInt(1691486100000),
			Open:        fixedpoint.NewFromFloat(29045.3),
			High:        fixedpoint.NewFromFloat(29228.56),
			Low:         fixedpoint.NewFromFloat(29045.3),
			Close:       fixedpoint.NewFromFloat(29228.56),
			Volume:      fixedpoint.NewFromFloat(9.265593),
			QuoteVolume: fixedpoint.NewFromFloat(270447.43520753),
			UsdtVolume:  fixedpoint.NewFromFloat(270447.43520753),
		},
		{
			Ts:          types.NewMillisecondTimestampFromInt(1691487000000),
			Open:        fixedpoint.NewFromFloat(29167.33),
			High:        fixedpoint.NewFromFloat(29229.08),
			Low:         fixedpoint.NewFromFloat(29000),
			Close:       fixedpoint.NewFromFloat(29045.3),
			Volume:      fixedpoint.NewFromFloat(9.295508),
			QuoteVolume: fixedpoint.NewFromFloat(270816.87513775),
			UsdtVolume:  fixedpoint.NewFromFloat(270447.43520753),
		},
	}

	expKlines := []types.KLine{
		{
			Exchange:    types.ExchangeBitget,
			Symbol:      symbol,
			StartTime:   types.Time(resp[0].Ts.Time()),
			EndTime:     types.Time(resp[0].Ts.Time().Add(interval.Duration() - time.Millisecond)),
			Interval:    interval,
			Open:        fixedpoint.NewFromFloat(29045.3),
			Close:       fixedpoint.NewFromFloat(29228.56),
			High:        fixedpoint.NewFromFloat(29228.56),
			Low:         fixedpoint.NewFromFloat(29045.3),
			Volume:      fixedpoint.NewFromFloat(9.265593),
			QuoteVolume: fixedpoint.NewFromFloat(270447.43520753),
			Closed:      false,
		},
		{
			Exchange:    types.ExchangeBitget,
			Symbol:      symbol,
			StartTime:   types.Time(resp[1].Ts.Time()),
			EndTime:     types.Time(resp[1].Ts.Time().Add(interval.Duration() - time.Millisecond)),
			Interval:    interval,
			Open:        fixedpoint.NewFromFloat(29167.33),
			Close:       fixedpoint.NewFromFloat(29045.3),
			High:        fixedpoint.NewFromFloat(29229.08),
			Low:         fixedpoint.NewFromFloat(29000),
			Volume:      fixedpoint.NewFromFloat(9.295508),
			QuoteVolume: fixedpoint.NewFromFloat(270816.87513775),
			Closed:      false,
		},
	}

	assert.Equal(t, toGlobalKLines(symbol, interval, resp), expKlines)
}

func Test_toGlobalTimeInForce(t *testing.T) {
	force, err := toGlobalTimeInForce(v2.OrderForceFOK)
	assert.NoError(t, err)
	assert.Equal(t, types.TimeInForceFOK, force)

	force, err = toGlobalTimeInForce(v2.OrderForceGTC)
	assert.NoError(t, err)
	assert.Equal(t, types.TimeInForceGTC, force)

	force, err = toGlobalTimeInForce(v2.OrderForcePostOnly)
	assert.NoError(t, err)
	assert.Equal(t, types.TimeInForceGTC, force)

	force, err = toGlobalTimeInForce(v2.OrderForceIOC)
	assert.NoError(t, err)
	assert.Equal(t, types.TimeInForceIOC, force)

	_, err = toGlobalTimeInForce("xxx")
	assert.ErrorContains(t, err, "xxx")
}

func TestOrder_processMarketBuyQuantity(t *testing.T) {
	t.Run("zero qty", func(t *testing.T) {
		o := Order{}
		for _, s := range []v2.OrderStatus{v2.OrderStatusLive, v2.OrderStatusNew, v2.OrderStatusInit, v2.OrderStatusCancelled} {
			o.Status = s
			qty, err := o.processMarketBuyQuantity()
			assert.NoError(t, err)
			assert.Equal(t, fixedpoint.Zero, qty)
		}
	})

	t.Run("calculate qty", func(t *testing.T) {
		o := Order{
			NewSize: fixedpoint.NewFromFloat(2),
			Trade: Trade{
				FillPrice: fixedpoint.NewFromFloat(1),
			},
			Status: v2.OrderStatusPartialFilled,
		}
		qty, err := o.processMarketBuyQuantity()
		assert.NoError(t, err)
		assert.Equal(t, fixedpoint.NewFromFloat(2), qty)
	})

	t.Run("return accumulated balance", func(t *testing.T) {
		o := Order{
			AccBaseVolume: fixedpoint.NewFromFloat(5),
			Status:        v2.OrderStatusFilled,
		}
		qty, err := o.processMarketBuyQuantity()
		assert.NoError(t, err)
		assert.Equal(t, fixedpoint.NewFromFloat(5), qty)
	})

	t.Run("unexpected status", func(t *testing.T) {
		o := Order{
			Status: "xxx",
		}
		_, err := o.processMarketBuyQuantity()
		assert.ErrorContains(t, err, "xxx")
	})
}

func TestOrder_toGlobalOrder(t *testing.T) {
	o := Order{
		Trade: Trade{
			FillPrice:   fixedpoint.NewFromFloat(0.49016),
			TradeId:     types.StrInt64(1107950490073112582),
			BaseVolume:  fixedpoint.NewFromFloat(33.6558),
			FillTime:    types.NewMillisecondTimestampFromInt(1699881902235),
			FillFee:     fixedpoint.NewFromFloat(-0.0336558),
			FillFeeCoin: "BGB",
			TradeScope:  "T",
		},
		InstId:           "BGBUSDT",
		OrderId:          types.StrInt64(1107950489998626816),
		ClientOrderId:    "cc73aab9-1e44-4022-8458-60d8c6a08753",
		NewSize:          fixedpoint.NewFromFloat(39.0),
		Notional:         fixedpoint.NewFromFloat(39.0),
		OrderType:        v2.OrderTypeMarket,
		Force:            v2.OrderForceGTC,
		Side:             v2.SideTypeBuy,
		AccBaseVolume:    fixedpoint.NewFromFloat(33.6558),
		PriceAvg:         fixedpoint.NewFromFloat(0.49016),
		Status:           v2.OrderStatusPartialFilled,
		CreatedTime:      types.NewMillisecondTimestampFromInt(1699881902217),
		UpdatedTime:      types.NewMillisecondTimestampFromInt(1699881902248),
		FeeDetail:        nil,
		EnterPointSource: "API",
	}

	// market buy example:
	//      {
	//         "instId":"BGBUSDT",
	//         "orderId":"1107950489998626816",
	//         "clientOid":"cc73aab9-1e44-4022-8458-60d8c6a08753",
	//         "newSize":"39.0000",
	//         "notional":"39.000000",
	//         "orderType":"market",
	//         "force":"gtc",
	//         "side":"buy",
	//         "fillPrice":"0.49016",
	//         "tradeId":"1107950490073112582",
	//         "baseVolume":"33.6558",
	//         "fillTime":"1699881902235",
	//         "fillFee":"-0.0336558",
	//         "fillFeeCoin":"BGB",
	//         "tradeScope":"T",
	//         "accBaseVolume":"33.6558",
	//         "priceAvg":"0.49016",
	//         "status":"partially_filled",
	//         "cTime":"1699881902217",
	//         "uTime":"1699881902248",
	//         "feeDetail":[
	//            {
	//               "feeCoin":"BGB",
	//               "fee":"-0.0336558"
	//            }
	//         ],
	//         "enterPointSource":"API"
	//      }
	t.Run("market buy", func(t *testing.T) {
		newO := o
		res, err := newO.toGlobalOrder()
		assert.NoError(t, err)
		assert.Equal(t, types.Order{
			SubmitOrder: types.SubmitOrder{
				ClientOrderID: "cc73aab9-1e44-4022-8458-60d8c6a08753",
				Symbol:        "BGBUSDT",
				Side:          types.SideTypeBuy,
				Type:          types.OrderTypeMarket,
				Quantity:      newO.NewSize.Div(newO.FillPrice),
				Price:         newO.PriceAvg,
				TimeInForce:   types.TimeInForceGTC,
			},
			Exchange:         types.ExchangeBitget,
			OrderID:          uint64(newO.OrderId),
			UUID:             strconv.FormatInt(int64(newO.OrderId), 10),
			Status:           types.OrderStatusPartiallyFilled,
			ExecutedQuantity: newO.AccBaseVolume,
			IsWorking:        newO.Status.IsWorking(),
			CreationTime:     types.Time(newO.CreatedTime),
			UpdateTime:       types.Time(newO.UpdatedTime),
		}, res)
	})

	// market sell example:
	//      {
	//         "instId":"BGBUSDT",
	//         "orderId":"1107940456212631553",
	//         "clientOid":"088bb971-858e-48e2-b503-85c3274edd89",
	//         "newSize":"285.0000",
	//         "orderType":"market",
	//         "force":"gtc",
	//         "side":"sell",
	//         "fillPrice":"0.48706",
	//         "tradeId":"1107940456278728706",
	//         "baseVolume":"22.5840",
	//         "fillTime":"1699879509992",
	//         "fillFee":"-0.01099976304",
	//         "fillFeeCoin":"USDT",
	//         "tradeScope":"T",
	//         "accBaseVolume":"45.1675",
	//         "priceAvg":"0.48706",
	//         "status":"partially_filled",
	//         "cTime":"1699879509976",
	//         "uTime":"1699879510007",
	//         "feeDetail":[
	//            {
	//               "feeCoin":"USDT",
	//               "fee":"-0.02199928255"
	//            }
	//         ],
	//         "enterPointSource":"API"
	//      }
	t.Run("market sell", func(t *testing.T) {
		newO := o
		newO.OrderType = v2.OrderTypeMarket
		newO.Side = v2.SideTypeSell

		res, err := newO.toGlobalOrder()
		assert.NoError(t, err)
		assert.Equal(t, types.Order{
			SubmitOrder: types.SubmitOrder{
				ClientOrderID: "cc73aab9-1e44-4022-8458-60d8c6a08753",
				Symbol:        "BGBUSDT",
				Side:          types.SideTypeSell,
				Type:          types.OrderTypeMarket,
				Quantity:      newO.NewSize,
				Price:         newO.PriceAvg,
				TimeInForce:   types.TimeInForceGTC,
			},
			Exchange:         types.ExchangeBitget,
			OrderID:          uint64(newO.OrderId),
			UUID:             strconv.FormatInt(int64(newO.OrderId), 10),
			Status:           types.OrderStatusPartiallyFilled,
			ExecutedQuantity: newO.AccBaseVolume,
			IsWorking:        newO.Status.IsWorking(),
			CreationTime:     types.Time(newO.CreatedTime),
			UpdateTime:       types.Time(newO.UpdatedTime),
		}, res)
	})

	// limit buy example:
	//      {
	//         "instId":"BGBUSDT",
	//         "orderId":"1107955329902481408",
	//         "clientOid":"c578164a-bf34-44ba-8bb7-a1538f33b1b8",
	//         "price":"0.49998",
	//         "newSize":"24.9990",
	//         "notional":"24.999000",
	//         "orderType":"limit",
	//         "force":"gtc",
	//         "side":"buy",
	//         "fillPrice":"0.49998",
	//         "tradeId":"1107955401758285828",
	//         "baseVolume":"15.9404",
	//         "fillTime":"1699883073272",
	//         "fillFee":"-0.0159404",
	//         "fillFeeCoin":"BGB",
	//         "tradeScope":"M",
	//         "accBaseVolume":"15.9404",
	//         "priceAvg":"0.49998",
	//         "status":"partially_filled",
	//         "cTime":"1699883056140",
	//         "uTime":"1699883073285",
	//         "feeDetail":[
	//            {
	//               "feeCoin":"BGB",
	//               "fee":"-0.0159404"
	//            }
	//         ],
	//         "enterPointSource":"API"
	//      }
	t.Run("limit buy", func(t *testing.T) {
		newO := o
		newO.Price = fixedpoint.NewFromFloat(0.49998)
		newO.OrderType = v2.OrderTypeLimit

		res, err := newO.toGlobalOrder()
		assert.NoError(t, err)
		assert.Equal(t, types.Order{
			SubmitOrder: types.SubmitOrder{
				ClientOrderID: "cc73aab9-1e44-4022-8458-60d8c6a08753",
				Symbol:        "BGBUSDT",
				Side:          types.SideTypeBuy,
				Type:          types.OrderTypeLimit,
				Quantity:      newO.NewSize,
				Price:         newO.Price,
				TimeInForce:   types.TimeInForceGTC,
			},
			Exchange:         types.ExchangeBitget,
			OrderID:          uint64(newO.OrderId),
			UUID:             strconv.FormatInt(int64(newO.OrderId), 10),
			Status:           types.OrderStatusPartiallyFilled,
			ExecutedQuantity: newO.AccBaseVolume,
			IsWorking:        newO.Status.IsWorking(),
			CreationTime:     types.Time(newO.CreatedTime),
			UpdateTime:       types.Time(newO.UpdatedTime),
		}, res)
	})

	// limit sell example:
	//      {
	//         "instId":"BGBUSDT",
	//         "orderId":"1107936497259417600",
	//         "clientOid":"02d4592e-091c-4b5a-aef3-6a7cf57b5e82",
	//         "price":"0.48710",
	//         "newSize":"280.0000",
	//         "orderType":"limit",
	//         "force":"gtc",
	//         "side":"sell",
	//         "fillPrice":"0.48710",
	//         "tradeId":"1107937053540556809",
	//         "baseVolume":"41.0593",
	//         "fillTime":"1699878698716",
	//         "fillFee":"-0.01999998503",
	//         "fillFeeCoin":"USDT",
	//         "tradeScope":"M",
	//         "accBaseVolume":"146.3209",
	//         "priceAvg":"0.48710",
	//         "status":"partially_filled",
	//         "cTime":"1699878566088",
	//         "uTime":"1699878698746",
	//         "feeDetail":[
	//            {
	//               "feeCoin":"USDT",
	//               "fee":"-0.07127291039"
	//            }
	//         ],
	//         "enterPointSource":"API"
	//      }
	t.Run("limit sell", func(t *testing.T) {
		newO := o
		newO.OrderType = v2.OrderTypeLimit
		newO.Side = v2.SideTypeSell
		newO.Price = fixedpoint.NewFromFloat(0.48710)

		res, err := newO.toGlobalOrder()
		assert.NoError(t, err)
		assert.Equal(t, types.Order{
			SubmitOrder: types.SubmitOrder{
				ClientOrderID: "cc73aab9-1e44-4022-8458-60d8c6a08753",
				Symbol:        "BGBUSDT",
				Side:          types.SideTypeSell,
				Type:          types.OrderTypeLimit,
				Quantity:      newO.NewSize,
				Price:         newO.Price,
				TimeInForce:   types.TimeInForceGTC,
			},
			Exchange:         types.ExchangeBitget,
			OrderID:          uint64(newO.OrderId),
			UUID:             strconv.FormatInt(int64(newO.OrderId), 10),
			Status:           types.OrderStatusPartiallyFilled,
			ExecutedQuantity: newO.AccBaseVolume,
			IsWorking:        newO.Status.IsWorking(),
			CreationTime:     types.Time(newO.CreatedTime),
			UpdateTime:       types.Time(newO.UpdatedTime),
		}, res)
	})

	t.Run("unexpected status", func(t *testing.T) {
		newO := o
		newO.Status = "xxx"
		_, err := newO.toGlobalOrder()
		assert.ErrorContains(t, err, "xxx")
	})

	t.Run("unexpected time-in-force", func(t *testing.T) {
		newO := o
		newO.Force = "xxx"
		_, err := newO.toGlobalOrder()
		assert.ErrorContains(t, err, "xxx")
	})

	t.Run("unexpected order type", func(t *testing.T) {
		newO := o
		newO.OrderType = "xxx"
		_, err := newO.toGlobalOrder()
		assert.ErrorContains(t, err, "xxx")
	})

	t.Run("unexpected side", func(t *testing.T) {
		newO := o
		newO.Side = "xxx"
		_, err := newO.toGlobalOrder()
		assert.ErrorContains(t, err, "xxx")
	})
}

func TestOrder_toGlobalTrade(t *testing.T) {
	// market buy example:
	//      {
	//         "instId":"BGBUSDT",
	//         "orderId":"1107950489998626816",
	//         "clientOid":"cc73aab9-1e44-4022-8458-60d8c6a08753",
	//         "newSize":"39.0000",
	//         "notional":"39.000000",
	//         "orderType":"market",
	//         "force":"gtc",
	//         "side":"buy",
	//         "fillPrice":"0.49016",
	//         "tradeId":"1107950490073112582",
	//         "baseVolume":"33.6558",
	//         "fillTime":"1699881902235",
	//         "fillFee":"-0.0336558",
	//         "fillFeeCoin":"BGB",
	//         "tradeScope":"T",
	//         "accBaseVolume":"33.6558",
	//         "priceAvg":"0.49016",
	//         "status":"partially_filled",
	//         "cTime":"1699881902217",
	//         "uTime":"1699881902248",
	//         "feeDetail":[
	//            {
	//               "feeCoin":"BGB",
	//               "fee":"-0.0336558"
	//            }
	//         ],
	//         "enterPointSource":"API"
	//      }
	o := Order{
		Trade: Trade{
			FillPrice:   fixedpoint.NewFromFloat(0.49016),
			TradeId:     types.StrInt64(1107950490073112582),
			BaseVolume:  fixedpoint.NewFromFloat(33.6558),
			FillTime:    types.NewMillisecondTimestampFromInt(1699881902235),
			FillFee:     fixedpoint.NewFromFloat(-0.0336558),
			FillFeeCoin: "BGB",
			TradeScope:  "T",
		},
		InstId:           "BGBUSDT",
		OrderId:          types.StrInt64(1107950489998626816),
		ClientOrderId:    "cc73aab9-1e44-4022-8458-60d8c6a08753",
		NewSize:          fixedpoint.NewFromFloat(39.0),
		Notional:         fixedpoint.NewFromFloat(39.0),
		OrderType:        v2.OrderTypeMarket,
		Force:            v2.OrderForceGTC,
		Side:             v2.SideTypeBuy,
		AccBaseVolume:    fixedpoint.NewFromFloat(33.6558),
		PriceAvg:         fixedpoint.NewFromFloat(0.49016),
		Status:           v2.OrderStatusPartialFilled,
		CreatedTime:      types.NewMillisecondTimestampFromInt(1699881902217),
		UpdatedTime:      types.NewMillisecondTimestampFromInt(1699881902248),
		FeeDetail:        nil,
		EnterPointSource: "API",
	}

	t.Run("succeeds", func(t *testing.T) {
		res, err := o.toGlobalTrade()
		assert.NoError(t, err)
		assert.Equal(t, types.Trade{
			ID:            uint64(o.TradeId),
			OrderID:       uint64(o.OrderId),
			Exchange:      types.ExchangeBitget,
			Price:         o.FillPrice,
			Quantity:      o.BaseVolume,
			QuoteQuantity: o.FillPrice.Mul(o.BaseVolume),
			Symbol:        "BGBUSDT",
			Side:          types.SideTypeBuy,
			IsBuyer:       true,
			IsMaker:       false,
			Time:          types.Time(o.FillTime),
			Fee:           o.FillFee.Abs(),
			FeeCurrency:   "BGB",
		}, res)
	})

	t.Run("unexpected trade scope", func(t *testing.T) {
		newO := o
		newO.TradeScope = "xxx"
		_, err := newO.toGlobalTrade()
		assert.ErrorContains(t, err, "xxx")
	})

	t.Run("unexpected side type", func(t *testing.T) {
		newO := o
		newO.Side = "xxx"
		_, err := newO.toGlobalTrade()
		assert.ErrorContains(t, err, "xxx")
	})

	t.Run("unexpected side type", func(t *testing.T) {
		newO := o
		newO.Status = "xxx"
		_, err := newO.toGlobalTrade()
		assert.ErrorContains(t, err, "xxx")
	})
}

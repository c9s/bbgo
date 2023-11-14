package bitget

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi"
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
	asset := bitgetapi.AccountAsset{
		CoinId:    2,
		CoinName:  "USDT",
		Available: fixedpoint.NewFromFloat(1.2),
		Frozen:    fixedpoint.NewFromFloat(0.5),
		Lock:      fixedpoint.NewFromFloat(0.5),
		UTime:     types.NewMillisecondTimestampFromInt(1622697148),
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
	//{
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
	//}
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
		Status:              v2.SymbolOnline,
		BuyLimitPriceRatio:  fixedpoint.NewFromFloat(0.05),
		SellLimitPriceRatio: fixedpoint.NewFromFloat(0.05),
	}

	exp := types.Market{
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
	//        "symbol": "BTCUSDT",
	//        "high24h": "24175.65",
	//        "low24h": "23677.75",
	//        "close": "24014.11",
	//        "quoteVol": "177689342.3025",
	//        "baseVol": "7421.5009",
	//        "usdtVol": "177689342.302407",
	//        "ts": "1660704288118",
	//        "buyOne": "24013.94",
	//        "sellOne": "24014.06",
	//        "bidSz": "0.0663",
	//        "askSz": "0.0119",
	//        "openUtc0": "23856.72",
	//        "changeUtc":"0.00301",
	//        "change":"0.00069"
	//    }
	ticker := bitgetapi.Ticker{
		Symbol:    "BTCUSDT",
		High24H:   fixedpoint.NewFromFloat(24175.65),
		Low24H:    fixedpoint.NewFromFloat(23677.75),
		Close:     fixedpoint.NewFromFloat(24014.11),
		QuoteVol:  fixedpoint.NewFromFloat(177689342.3025),
		BaseVol:   fixedpoint.NewFromFloat(7421.5009),
		UsdtVol:   fixedpoint.NewFromFloat(177689342.302407),
		Ts:        types.NewMillisecondTimestampFromInt(1660704288118),
		BuyOne:    fixedpoint.NewFromFloat(24013.94),
		SellOne:   fixedpoint.NewFromFloat(24014.06),
		BidSz:     fixedpoint.NewFromFloat(0.0663),
		AskSz:     fixedpoint.NewFromFloat(0.0119),
		OpenUtc0:  fixedpoint.NewFromFloat(23856.72),
		ChangeUtc: fixedpoint.NewFromFloat(0.00301),
		Change:    fixedpoint.NewFromFloat(0.00069),
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
			CTime:            types.NewMillisecondTimestampFromInt(1660704288118),
			UTime:            types.NewMillisecondTimestampFromInt(1660704288118),
		}
	)

	t.Run("succeeds", func(t *testing.T) {
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
			CTime:            types.NewMillisecondTimestampFromInt(1660704288118),
			UTime:            types.NewMillisecondTimestampFromInt(1660704288118),
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
	//   "size":"5",
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
	//}
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
		TradeScope: v2.TradeTaker,
		CTime:      types.NewMillisecondTimestampFromInt(1699020564676),
		UTime:      types.NewMillisecondTimestampFromInt(1699020564687),
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
			UTime:          types.NewMillisecondTimestampFromInt(1699020564676),
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

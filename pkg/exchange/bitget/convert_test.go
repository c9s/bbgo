package bitget

import (
	"strconv"
	"testing"

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
	//            "symbol":"BTCUSDT_SPBL",
	//            "symbolName":"BTCUSDT",
	//            "baseCoin":"BTC",
	//            "quoteCoin":"USDT",
	//            "minTradeAmount":"0.0001",
	//            "maxTradeAmount":"10000",
	//            "takerFeeRate":"0.001",
	//            "makerFeeRate":"0.001",
	//            "priceScale":"4",
	//            "quantityScale":"8",
	//            "minTradeUSDT":"5",
	//            "status":"online",
	//            "buyLimitPriceRatio": "0.05",
	//            "sellLimitPriceRatio": "0.05"
	//        }
	inst := bitgetapi.Symbol{
		Symbol:              "BTCUSDT_SPBL",
		SymbolName:          "BTCUSDT",
		BaseCoin:            "BTC",
		QuoteCoin:           "USDT",
		MinTradeAmount:      fixedpoint.NewFromFloat(0.0001),
		MaxTradeAmount:      fixedpoint.NewFromFloat(10000),
		TakerFeeRate:        fixedpoint.NewFromFloat(0.001),
		MakerFeeRate:        fixedpoint.NewFromFloat(0.001),
		PriceScale:          fixedpoint.NewFromFloat(4),
		QuantityScale:       fixedpoint.NewFromFloat(8),
		MinTradeUSDT:        fixedpoint.NewFromFloat(5),
		Status:              bitgetapi.SymbolOnline,
		BuyLimitPriceRatio:  fixedpoint.NewFromFloat(0.05),
		SellLimitPriceRatio: fixedpoint.NewFromFloat(0.05),
	}

	exp := types.Market{
		Symbol:          inst.SymbolName,
		LocalSymbol:     inst.Symbol,
		PricePrecision:  4,
		VolumePrecision: 8,
		QuoteCurrency:   inst.QuoteCoin,
		BaseCurrency:    inst.BaseCoin,
		MinNotional:     inst.MinTradeUSDT,
		MinAmount:       inst.MinTradeUSDT,
		MinQuantity:     inst.MinTradeAmount,
		MaxQuantity:     inst.MaxTradeAmount,
		StepSize:        fixedpoint.NewFromFloat(0.00000001),
		MinPrice:        fixedpoint.Zero,
		MaxPrice:        fixedpoint.Zero,
		TickSize:        fixedpoint.NewFromFloat(0.0001),
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

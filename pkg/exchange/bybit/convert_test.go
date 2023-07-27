package bybit

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestToGlobalMarket(t *testing.T) {
	// sample:
	//{
	//	"Symbol": "BTCUSDT",
	//	"BaseCoin": "BTC",
	//	"QuoteCoin": "USDT",
	//	"Innovation": 0,
	//	"Status": "Trading",
	//	"MarginTrading": "both",
	//	"LotSizeFilter": {
	//	"BasePrecision": 0.000001,
	//		"QuotePrecision": 0.00000001,
	//		"MinOrderQty": 0.000048,
	//		"MaxOrderQty": 71.73956243,
	//		"MinOrderAmt": 1,
	//		"MaxOrderAmt": 2000000
	//	},
	//	"PriceFilter": {
	//		"TickSize": 0.01
	//	}
	//}
	inst := bybitapi.Instrument{
		Symbol:        "BTCUSDT",
		BaseCoin:      "BTC",
		QuoteCoin:     "USDT",
		Innovation:    "0",
		Status:        bybitapi.StatusTrading,
		MarginTrading: "both",
		LotSizeFilter: struct {
			BasePrecision  fixedpoint.Value `json:"basePrecision"`
			QuotePrecision fixedpoint.Value `json:"quotePrecision"`
			MinOrderQty    fixedpoint.Value `json:"minOrderQty"`
			MaxOrderQty    fixedpoint.Value `json:"maxOrderQty"`
			MinOrderAmt    fixedpoint.Value `json:"minOrderAmt"`
			MaxOrderAmt    fixedpoint.Value `json:"maxOrderAmt"`
		}{
			BasePrecision:  fixedpoint.NewFromFloat(0.000001),
			QuotePrecision: fixedpoint.NewFromFloat(0.00000001),
			MinOrderQty:    fixedpoint.NewFromFloat(0.000048),
			MaxOrderQty:    fixedpoint.NewFromFloat(71.73956243),
			MinOrderAmt:    fixedpoint.NewFromInt(1),
			MaxOrderAmt:    fixedpoint.NewFromInt(2000000),
		},
		PriceFilter: struct {
			TickSize fixedpoint.Value `json:"tickSize"`
		}{
			TickSize: fixedpoint.NewFromFloat(0.01),
		},
	}

	exp := types.Market{
		Symbol:          inst.Symbol,
		LocalSymbol:     inst.Symbol,
		PricePrecision:  int(math.Log10(inst.LotSizeFilter.QuotePrecision.Float64())),
		VolumePrecision: int(math.Log10(inst.LotSizeFilter.BasePrecision.Float64())),
		QuoteCurrency:   inst.QuoteCoin,
		BaseCurrency:    inst.BaseCoin,
		MinNotional:     inst.LotSizeFilter.MinOrderAmt,
		MinAmount:       inst.LotSizeFilter.MinOrderAmt,
		MinQuantity:     inst.LotSizeFilter.MinOrderQty,
		MaxQuantity:     inst.LotSizeFilter.MaxOrderQty,
		StepSize:        inst.LotSizeFilter.BasePrecision,
		MinPrice:        inst.LotSizeFilter.MinOrderAmt,
		MaxPrice:        inst.LotSizeFilter.MaxOrderAmt,
		TickSize:        inst.PriceFilter.TickSize,
	}

	assert.Equal(t, toGlobalMarket(inst), exp)
}

func TestToGlobalTicker(t *testing.T) {
	// sample
	//{
	// 	  "symbol": "BTCUSDT",
	//    "bid1Price": "28995.98",
	//    "bid1Size": "4.741552",
	//    "ask1Price": "28995.99",
	//    "ask1Size": "0.16075",
	//    "lastPrice": "28994",
	//    "prevPrice24h": "29900",
	//    "price24hPcnt": "-0.0303",
	//    "highPrice24h": "30344.78",
	//    "lowPrice24h": "28948.87",
	//    "turnover24h": "184705500.13172874",
	//    "volume24h": "6240.807096",
	//    "usdIndexPrice": "28977.82001643"
	//}
	ticker := bybitapi.Ticker{
		Symbol:        "BTCUSDT",
		Bid1Price:     fixedpoint.NewFromFloat(28995.98),
		Bid1Size:      fixedpoint.NewFromFloat(4.741552),
		Ask1Price:     fixedpoint.NewFromFloat(28995.99),
		Ask1Size:      fixedpoint.NewFromFloat(0.16075),
		LastPrice:     fixedpoint.NewFromFloat(28994),
		PrevPrice24H:  fixedpoint.NewFromFloat(29900),
		Price24HPcnt:  fixedpoint.NewFromFloat(-0.0303),
		HighPrice24H:  fixedpoint.NewFromFloat(30344.78),
		LowPrice24H:   fixedpoint.NewFromFloat(28948.87),
		Turnover24H:   fixedpoint.NewFromFloat(184705500.13172874),
		Volume24H:     fixedpoint.NewFromFloat(6240.807096),
		UsdIndexPrice: fixedpoint.NewFromFloat(28977.82001643),
	}

	timeNow := time.Now()

	exp := types.Ticker{
		Time:   timeNow,
		Volume: ticker.Volume24H,
		Last:   ticker.LastPrice,
		Open:   ticker.PrevPrice24H,
		High:   ticker.HighPrice24H,
		Low:    ticker.LowPrice24H,
		Buy:    ticker.Bid1Price,
		Sell:   ticker.Ask1Price,
	}

	assert.Equal(t, toGlobalTicker(ticker, timeNow), exp)
}

func TestToGlobalOrder(t *testing.T) {
	// sample: partialFilled
	//{
	//  "OrderId": 1472539279335923200,
	//  "OrderLinkId": 1690276361150,
	//  "BlockTradeId": null,
	//  "Symbol": "DOTUSDT",
	//  "Price": 7.278,
	//  "Qty": 0.8,
	//  "Side": "Sell",
	//  "IsLeverage": 0,
	//  "PositionIdx": 0,
	//  "OrderStatus": "PartiallyFilled",
	//  "CancelType": "UNKNOWN",
	//  "RejectReason": null,
	//  "AvgPrice": 7.278,
	//  "LeavesQty": 0,
	//  "LeavesValue": 0,
	//  "CumExecQty": 0.5,
	//  "CumExecValue": 0,
	//  "CumExecFee": 0,
	//  "TimeInForce": "GTC",
	//  "OrderType": "Limit",
	//  "StopOrderType": null,
	//  "OrderIv": null,
	//  "TriggerPrice": 0,
	//  "TakeProfit": 0,
	//  "StopLoss": 0,
	//  "TpTriggerBy": null,
	//  "SlTriggerBy": null,
	//  "TriggerDirection": 0,
	//  "TriggerBy": null,
	//  "LastPriceOnCreated": null,
	//  "ReduceOnly": false,
	//  "CloseOnTrigger": false,
	//  "SmpType": "None",
	//  "SmpGroup": 0,
	//  "SmpOrderId": null,
	//  "TpslMode": null,
	//  "TpLimitPrice": null,
	//  "SlLimitPrice": null,
	//  "PlaceType": null,
	//  "CreatedTime": "2023-07-25 17:12:41.325 +0800 CST",
	//  "UpdatedTime": "2023-07-25 17:12:57.868 +0800 CST"
	//}
	timeNow := time.Now()
	openOrder := bybitapi.Order{
		OrderId:            "1472539279335923200",
		OrderLinkId:        "1690276361150",
		BlockTradeId:       "",
		Symbol:             "DOTUSDT",
		Price:              fixedpoint.NewFromFloat(7.278),
		Qty:                fixedpoint.NewFromFloat(0.8),
		Side:               bybitapi.SideSell,
		IsLeverage:         "0",
		PositionIdx:        0,
		OrderStatus:        bybitapi.OrderStatusPartiallyFilled,
		CancelType:         "UNKNOWN",
		RejectReason:       "",
		AvgPrice:           fixedpoint.NewFromFloat(7.728),
		LeavesQty:          fixedpoint.NewFromFloat(0),
		LeavesValue:        fixedpoint.NewFromFloat(0),
		CumExecQty:         fixedpoint.NewFromFloat(0.5),
		CumExecValue:       fixedpoint.NewFromFloat(0),
		CumExecFee:         fixedpoint.NewFromFloat(0),
		TimeInForce:        "GTC",
		OrderType:          bybitapi.OrderTypeLimit,
		StopOrderType:      "",
		OrderIv:            "",
		TriggerPrice:       fixedpoint.NewFromFloat(0),
		TakeProfit:         fixedpoint.NewFromFloat(0),
		StopLoss:           fixedpoint.NewFromFloat(0),
		TpTriggerBy:        "",
		SlTriggerBy:        "",
		TriggerDirection:   0,
		TriggerBy:          "",
		LastPriceOnCreated: "",
		ReduceOnly:         false,
		CloseOnTrigger:     false,
		SmpType:            "None",
		SmpGroup:           0,
		SmpOrderId:         "",
		TpslMode:           "",
		TpLimitPrice:       "",
		SlLimitPrice:       "",
		PlaceType:          "",
		CreatedTime:        types.MillisecondTimestamp(timeNow),
		UpdatedTime:        types.MillisecondTimestamp(timeNow),
	}
	side, err := toGlobalSideType(openOrder.Side)
	assert.NoError(t, err)
	orderType, err := toGlobalOrderType(openOrder.OrderType)
	assert.NoError(t, err)
	tif, err := toGlobalTimeInForce(openOrder.TimeInForce)
	assert.NoError(t, err)
	status, err := toGlobalOrderStatus(openOrder.OrderStatus)
	assert.NoError(t, err)
	working, err := isWorking(openOrder.OrderStatus)
	assert.NoError(t, err)

	exp := types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: openOrder.OrderLinkId,
			Symbol:        openOrder.Symbol,
			Side:          side,
			Type:          orderType,
			Quantity:      openOrder.Qty,
			Price:         openOrder.Price,
			TimeInForce:   tif,
		},
		Exchange:         types.ExchangeBybit,
		OrderID:          hashStringID(openOrder.OrderId),
		UUID:             openOrder.OrderId,
		Status:           status,
		ExecutedQuantity: openOrder.CumExecQty,
		IsWorking:        working,
		CreationTime:     types.Time(timeNow),
		UpdateTime:       types.Time(timeNow),
		IsFutures:        false,
		IsMargin:         false,
		IsIsolated:       false,
	}
	res, err := toGlobalOrder(openOrder)
	assert.NoError(t, err)
	assert.Equal(t, res, &exp)
}

func TestToGlobalSideType(t *testing.T) {
	res, err := toGlobalSideType(bybitapi.SideBuy)
	assert.NoError(t, err)
	assert.Equal(t, types.SideTypeBuy, res)

	res, err = toGlobalSideType(bybitapi.SideSell)
	assert.NoError(t, err)
	assert.Equal(t, types.SideTypeSell, res)

	res, err = toGlobalSideType("GG")
	assert.Error(t, err)
}

func TestToGlobalOrderType(t *testing.T) {
	res, err := toGlobalOrderType(bybitapi.OrderTypeMarket)
	assert.NoError(t, err)
	assert.Equal(t, types.OrderTypeMarket, res)

	res, err = toGlobalOrderType(bybitapi.OrderTypeLimit)
	assert.NoError(t, err)
	assert.Equal(t, types.OrderTypeLimit, res)

	res, err = toGlobalOrderType("GG")
	assert.Error(t, err)
}

func TestToGlobalTimeInForce(t *testing.T) {
	res, err := toGlobalTimeInForce(bybitapi.TimeInForceGTC)
	assert.NoError(t, err)
	assert.Equal(t, types.TimeInForceGTC, res)

	res, err = toGlobalTimeInForce(bybitapi.TimeInForceIOC)
	assert.NoError(t, err)
	assert.Equal(t, types.TimeInForceIOC, res)

	res, err = toGlobalTimeInForce(bybitapi.TimeInForceFOK)
	assert.NoError(t, err)
	assert.Equal(t, types.TimeInForceFOK, res)

	res, err = toGlobalTimeInForce("GG")
	assert.Error(t, err)
}

func TestToGlobalOrderStatus(t *testing.T) {
	t.Run("New", func(t *testing.T) {
		res, err := toGlobalOrderStatus(bybitapi.OrderStatusNew)
		assert.NoError(t, err)
		assert.Equal(t, types.OrderStatusNew, res)

		res, err = toGlobalOrderStatus(bybitapi.OrderStatusActive)
		assert.NoError(t, err)
		assert.Equal(t, types.OrderStatusNew, res)
	})

	t.Run("Filled", func(t *testing.T) {
		res, err := toGlobalOrderStatus(bybitapi.OrderStatusFilled)
		assert.NoError(t, err)
		assert.Equal(t, types.OrderStatusFilled, res)
	})

	t.Run("PartiallyFilled", func(t *testing.T) {
		res, err := toGlobalOrderStatus(bybitapi.OrderStatusPartiallyFilled)
		assert.NoError(t, err)
		assert.Equal(t, types.OrderStatusPartiallyFilled, res)
	})

	t.Run("OrderStatusCanceled", func(t *testing.T) {
		res, err := toGlobalOrderStatus(bybitapi.OrderStatusCancelled)
		assert.NoError(t, err)
		assert.Equal(t, types.OrderStatusCanceled, res)

		res, err = toGlobalOrderStatus(bybitapi.OrderStatusPartiallyFilledCanceled)
		assert.NoError(t, err)
		assert.Equal(t, types.OrderStatusCanceled, res)

		res, err = toGlobalOrderStatus(bybitapi.OrderStatusDeactivated)
		assert.NoError(t, err)
		assert.Equal(t, types.OrderStatusCanceled, res)
	})

	t.Run("OrderStatusRejected", func(t *testing.T) {
		res, err := toGlobalOrderStatus(bybitapi.OrderStatusRejected)
		assert.NoError(t, err)
		assert.Equal(t, types.OrderStatusRejected, res)
	})
}

func TestIsWorking(t *testing.T) {
	for _, s := range bybitapi.AllOrderStatuses {
		res, err := isWorking(s)
		assert.NoError(t, err)
		if res {
			gos, err := toGlobalOrderStatus(s)
			assert.NoError(t, err)
			assert.True(t, gos == types.OrderStatusNew || gos == types.OrderStatusPartiallyFilled)
		}
	}
}

func Test_toLocalOrderType(t *testing.T) {
	orderType, err := toLocalOrderType(types.OrderTypeLimit)
	assert.NoError(t, err)
	assert.Equal(t, bybitapi.OrderTypeLimit, orderType)

	orderType, err = toLocalOrderType(types.OrderTypeMarket)
	assert.NoError(t, err)
	assert.Equal(t, bybitapi.OrderTypeMarket, orderType)

	orderType, err = toLocalOrderType("wrong type")
	assert.Error(t, fmt.Errorf("order type %s not supported", "wrong side"), err)
	assert.Equal(t, bybitapi.OrderType(""), orderType)
}

func Test_toLocalSide(t *testing.T) {
	side, err := toLocalSide(types.SideTypeSell)
	assert.NoError(t, err)
	assert.Equal(t, bybitapi.SideSell, side)

	side, err = toLocalSide(types.SideTypeBuy)
	assert.NoError(t, err)
	assert.Equal(t, bybitapi.SideBuy, side)

	side, err = toLocalSide("wrong side")
	assert.Error(t, fmt.Errorf("side type %s not supported", "wrong side"), err)
	assert.Equal(t, bybitapi.Side(""), side)
}

package bybit

import (
	"fmt"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	v3 "github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi/v3"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
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

func Test_processMarketBuyQuantity(t *testing.T) {
	t.Run("websocket event", func(t *testing.T) {
		t.Run("Market/Buy/OrderStatusPartiallyFilled", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:          fixedpoint.NewFromFloat(5),
				OrderType:    bybitapi.OrderTypeMarket,
				Side:         bybitapi.SideBuy,
				CumExecValue: fixedpoint.NewFromFloat(200),
				CumExecQty:   fixedpoint.NewFromFloat(2),
				OrderStatus:  bybitapi.OrderStatusPartiallyFilled,
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, o.Qty.Div(o.CumExecValue.Div(o.CumExecQty)), res)
		})

		t.Run("Market/Buy/OrderStatusPartiallyFilledCanceled", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:          fixedpoint.NewFromFloat(5),
				OrderType:    bybitapi.OrderTypeMarket,
				Side:         bybitapi.SideBuy,
				CumExecValue: fixedpoint.NewFromFloat(200),
				CumExecQty:   fixedpoint.NewFromFloat(2),
				OrderStatus:  bybitapi.OrderStatusPartiallyFilled,
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, o.Qty.Div(o.CumExecValue.Div(o.CumExecQty)), res)
		})

		t.Run("Market/Buy/OrderStatusFilled", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:          fixedpoint.NewFromFloat(5),
				OrderType:    bybitapi.OrderTypeMarket,
				Side:         bybitapi.SideBuy,
				CumExecValue: fixedpoint.NewFromFloat(200),
				CumExecQty:   fixedpoint.NewFromFloat(2),
				OrderStatus:  bybitapi.OrderStatusFilled,
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, o.CumExecQty, res)
		})

		t.Run("Market/Buy/OrderStatusCreated", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:          fixedpoint.NewFromFloat(5),
				OrderType:    bybitapi.OrderTypeMarket,
				Side:         bybitapi.SideBuy,
				CumExecValue: fixedpoint.NewFromFloat(200),
				CumExecQty:   fixedpoint.NewFromFloat(2),
				OrderStatus:  bybitapi.OrderStatusCreated,
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, fixedpoint.Zero, res)
		})

		t.Run("Market/Buy/OrderStatusNew", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:          fixedpoint.NewFromFloat(5),
				OrderType:    bybitapi.OrderTypeMarket,
				Side:         bybitapi.SideBuy,
				CumExecValue: fixedpoint.NewFromFloat(200),
				CumExecQty:   fixedpoint.NewFromFloat(2),
				OrderStatus:  bybitapi.OrderStatusNew,
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, fixedpoint.Zero, res)
		})

		t.Run("Market/Buy/OrderStatusRejected", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:          fixedpoint.NewFromFloat(5),
				OrderType:    bybitapi.OrderTypeMarket,
				Side:         bybitapi.SideBuy,
				CumExecValue: fixedpoint.NewFromFloat(200),
				CumExecQty:   fixedpoint.NewFromFloat(2),
				OrderStatus:  bybitapi.OrderStatusRejected,
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, fixedpoint.Zero, res)
		})

		t.Run("Market/Buy/OrderStatusCanceled", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:          fixedpoint.NewFromFloat(5),
				OrderType:    bybitapi.OrderTypeMarket,
				Side:         bybitapi.SideBuy,
				CumExecValue: fixedpoint.NewFromFloat(200),
				CumExecQty:   fixedpoint.NewFromFloat(2),
				OrderStatus:  bybitapi.OrderStatusCancelled,
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, o.Qty, res)
		})

		t.Run("Market/Buy/Unexpected status", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:          fixedpoint.NewFromFloat(5),
				OrderType:    bybitapi.OrderTypeMarket,
				Side:         bybitapi.SideBuy,
				CumExecValue: fixedpoint.NewFromFloat(200),
				CumExecQty:   fixedpoint.NewFromFloat(2),
				OrderStatus:  bybitapi.OrderStatus("unexpected"),
			}
			res, err := processMarketBuyQuantity(o)
			assert.Error(t, err)
			assert.Equal(t, fmt.Errorf("unexpected order status: %s", o.OrderStatus), err)
			assert.Equal(t, fixedpoint.Zero, res)
		})

		t.Run("Market/Buy/CumExecQty zero", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:          fixedpoint.NewFromFloat(5),
				OrderType:    bybitapi.OrderTypeMarket,
				Side:         bybitapi.SideBuy,
				CumExecValue: fixedpoint.NewFromFloat(200),
				CumExecQty:   fixedpoint.Zero,
				OrderStatus:  bybitapi.OrderStatusPartiallyFilled,
			}
			res, err := processMarketBuyQuantity(o)
			assert.Error(t, err)
			assert.Equal(t, fmt.Errorf("CumExecQty shouldn't be zero"), err)
			assert.Equal(t, fixedpoint.Zero, res)
		})

		t.Run("Market/Sell", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:       fixedpoint.NewFromFloat(5.55),
				OrderType: bybitapi.OrderTypeMarket,
				Side:      bybitapi.SideSell,
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, o.Qty, res)
		})

		t.Run("Limit/Buy", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:       fixedpoint.NewFromFloat(5.55),
				OrderType: bybitapi.OrderTypeLimit,
				Side:      bybitapi.SideBuy,
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, o.Qty, res)
		})

		t.Run("Limit/Sell", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:       fixedpoint.NewFromFloat(5.55),
				OrderType: bybitapi.OrderTypeLimit,
				Side:      bybitapi.SideSell,
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, o.Qty, res)
		})
	})

	t.Run("Restful API", func(t *testing.T) {
		t.Run("Market/Buy/OrderStatusPartiallyFilled", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:         fixedpoint.NewFromFloat(200),
				OrderType:   bybitapi.OrderTypeMarket,
				Side:        bybitapi.SideBuy,
				AvgPrice:    fixedpoint.NewFromFloat(25000),
				OrderStatus: bybitapi.OrderStatusPartiallyFilled,
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, o.Qty.Div(o.AvgPrice), res)
		})

		t.Run("Market/Buy/OrderStatusPartiallyFilledCanceled", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:         fixedpoint.NewFromFloat(200),
				OrderType:   bybitapi.OrderTypeMarket,
				Side:        bybitapi.SideBuy,
				AvgPrice:    fixedpoint.NewFromFloat(25000),
				OrderStatus: bybitapi.OrderStatusPartiallyFilledCanceled,
				CumExecQty:  fixedpoint.NewFromFloat(0.002),
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, o.CumExecQty, res)
		})

		t.Run("Market/Buy/OrderStatusFilled", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:         fixedpoint.NewFromFloat(200),
				OrderType:   bybitapi.OrderTypeMarket,
				Side:        bybitapi.SideBuy,
				AvgPrice:    fixedpoint.NewFromFloat(25000),
				OrderStatus: bybitapi.OrderStatusFilled,
				CumExecQty:  fixedpoint.NewFromFloat(0.002),
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, o.CumExecQty, res)
		})

		t.Run("Market/Buy/OrderStatusCreated", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:         fixedpoint.NewFromFloat(200),
				OrderType:   bybitapi.OrderTypeMarket,
				Side:        bybitapi.SideBuy,
				AvgPrice:    fixedpoint.NewFromFloat(25000),
				OrderStatus: bybitapi.OrderStatusCreated,
				CumExecQty:  fixedpoint.NewFromFloat(0.002),
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, fixedpoint.Zero, res)
		})

		t.Run("Market/Buy/OrderStatusNew", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:         fixedpoint.NewFromFloat(200),
				OrderType:   bybitapi.OrderTypeMarket,
				Side:        bybitapi.SideBuy,
				AvgPrice:    fixedpoint.NewFromFloat(25000),
				OrderStatus: bybitapi.OrderStatusNew,
				CumExecQty:  fixedpoint.NewFromFloat(0.002),
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, fixedpoint.Zero, res)
		})

		t.Run("Market/Buy/OrderStatusRejected", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:         fixedpoint.NewFromFloat(200),
				OrderType:   bybitapi.OrderTypeMarket,
				Side:        bybitapi.SideBuy,
				AvgPrice:    fixedpoint.NewFromFloat(25000),
				OrderStatus: bybitapi.OrderStatusRejected,
				CumExecQty:  fixedpoint.NewFromFloat(0.002),
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, fixedpoint.Zero, res)
		})

		t.Run("Market/Buy/OrderStatusCanceled", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:         fixedpoint.NewFromFloat(200),
				OrderType:   bybitapi.OrderTypeMarket,
				Side:        bybitapi.SideBuy,
				AvgPrice:    fixedpoint.NewFromFloat(25000),
				OrderStatus: bybitapi.OrderStatusCancelled,
				CumExecQty:  fixedpoint.NewFromFloat(0.002),
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, o.Qty, res)
		})

		t.Run("Market/Buy/AvgPrice zero", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:         fixedpoint.NewFromFloat(200),
				OrderType:   bybitapi.OrderTypeMarket,
				Side:        bybitapi.SideBuy,
				AvgPrice:    fixedpoint.Zero,
				OrderStatus: bybitapi.OrderStatusPartiallyFilled,
				CumExecQty:  fixedpoint.NewFromFloat(0.002),
			}
			res, err := processMarketBuyQuantity(o)
			assert.Error(t, err)
			assert.Equal(t, fmt.Errorf("AvgPrice shouldn't be zero"), err)
			assert.Equal(t, fixedpoint.Zero, res)
		})

		t.Run("Market/Sell", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:       fixedpoint.NewFromFloat(5.55),
				OrderType: bybitapi.OrderTypeMarket,
				Side:      bybitapi.SideSell,
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, o.Qty, res)
		})

		t.Run("Limit/Buy", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:       fixedpoint.NewFromFloat(5.55),
				OrderType: bybitapi.OrderTypeLimit,
				Side:      bybitapi.SideBuy,
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, o.Qty, res)
		})

		t.Run("Limit/Sell", func(t *testing.T) {
			o := bybitapi.Order{
				Qty:       fixedpoint.NewFromFloat(5.55),
				OrderType: bybitapi.OrderTypeLimit,
				Side:      bybitapi.SideSell,
			}
			res, err := processMarketBuyQuantity(o)
			assert.NoError(t, err)
			assert.Equal(t, o.Qty, res)
		})
	})
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
	status, err := toGlobalOrderStatus(openOrder.OrderStatus, openOrder.Side, openOrder.OrderType)
	assert.NoError(t, err)
	orderIdNum, err := strconv.ParseUint(openOrder.OrderId, 10, 64)
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
		OrderID:          orderIdNum,
		UUID:             openOrder.OrderId,
		Status:           status,
		ExecutedQuantity: openOrder.CumExecQty,
		IsWorking:        status == types.OrderStatusNew || status == types.OrderStatusPartiallyFilled,
		CreationTime:     types.Time(openOrder.CreatedTime),
		UpdateTime:       types.Time(openOrder.UpdatedTime),
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

func Test_toGlobalOrderStatus(t *testing.T) {
	t.Run("market/buy", func(t *testing.T) {
		res, err := toGlobalOrderStatus(bybitapi.OrderStatusPartiallyFilledCanceled, bybitapi.SideBuy, bybitapi.OrderTypeMarket)
		assert.NoError(t, err)
		assert.Equal(t, types.OrderStatusFilled, res)
	})

	t.Run("limit/buy", func(t *testing.T) {
		res, err := toGlobalOrderStatus(bybitapi.OrderStatusPartiallyFilledCanceled, bybitapi.SideBuy, bybitapi.OrderTypeLimit)
		assert.NoError(t, err)
		assert.Equal(t, types.OrderStatusCanceled, res)
	})

	t.Run("limit/sell", func(t *testing.T) {
		res, err := toGlobalOrderStatus(bybitapi.OrderStatusPartiallyFilledCanceled, bybitapi.SideSell, bybitapi.OrderTypeLimit)
		assert.NoError(t, err)
		assert.Equal(t, types.OrderStatusCanceled, res)
	})
}

func Test_processOtherOrderStatus(t *testing.T) {
	t.Run("New", func(t *testing.T) {
		res, err := processOtherOrderStatus(bybitapi.OrderStatusNew)
		assert.NoError(t, err)
		assert.Equal(t, types.OrderStatusNew, res)

		res, err = processOtherOrderStatus(bybitapi.OrderStatusActive)
		assert.NoError(t, err)
		assert.Equal(t, types.OrderStatusNew, res)
	})

	t.Run("Filled", func(t *testing.T) {
		res, err := processOtherOrderStatus(bybitapi.OrderStatusFilled)
		assert.NoError(t, err)
		assert.Equal(t, types.OrderStatusFilled, res)
	})

	t.Run("PartiallyFilled", func(t *testing.T) {
		res, err := processOtherOrderStatus(bybitapi.OrderStatusPartiallyFilled)
		assert.NoError(t, err)
		assert.Equal(t, types.OrderStatusPartiallyFilled, res)
	})

	t.Run("OrderStatusCanceled", func(t *testing.T) {
		res, err := processOtherOrderStatus(bybitapi.OrderStatusCancelled)
		assert.NoError(t, err)
		assert.Equal(t, types.OrderStatusCanceled, res)

		res, err = processOtherOrderStatus(bybitapi.OrderStatusDeactivated)
		assert.NoError(t, err)
		assert.Equal(t, types.OrderStatusCanceled, res)
	})

	t.Run("OrderStatusRejected", func(t *testing.T) {
		res, err := processOtherOrderStatus(bybitapi.OrderStatusRejected)
		assert.NoError(t, err)
		assert.Equal(t, types.OrderStatusRejected, res)
	})

	t.Run("OrderStatusPartiallyFilledCanceled", func(t *testing.T) {
		res, err := processOtherOrderStatus(bybitapi.OrderStatusPartiallyFilledCanceled)
		assert.Equal(t, types.OrderStatus(bybitapi.OrderStatusPartiallyFilledCanceled), res)
		assert.Error(t, err)
		assert.Equal(t, fmt.Errorf("unexpected order status: %s", bybitapi.OrderStatusPartiallyFilledCanceled), err)
	})
}

func Test_toLocalOrderType(t *testing.T) {
	orderType, err := toLocalOrderType(types.OrderTypeLimit)
	assert.NoError(t, err)
	assert.Equal(t, bybitapi.OrderTypeLimit, orderType)

	orderType, err = toLocalOrderType(types.OrderTypeMarket)
	assert.NoError(t, err)
	assert.Equal(t, bybitapi.OrderTypeMarket, orderType)

	orderType, err = toLocalOrderType("wrong type")
	assert.Equal(t, fmt.Errorf("order type wrong type not supported"), err)
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
	assert.Equal(t, fmt.Errorf("side type %s not supported", "wrong side"), err)
	assert.Equal(t, bybitapi.Side(""), side)
}

func Test_toGlobalTrade(t *testing.T) {
	/* sample: trade
	{
		"Symbol":"BTCUSDT",
		"Id":"1474200510090276864",
		"OrderId":"1474200270671015936",
		"TradeId":"2100000000031181772",
		"OrderPrice":"27628",
		"OrderQty":"0.007959",
		"ExecFee":"0.21989125",
		"FeeTokenId":"USDT",
		"CreatTime":"2023-07-28 00:13:15.457 +0800 CST",
		"IsBuyer":"1",
		"IsMaker":"0",
		"MatchOrderId":"5760912963729109504",
		"MakerRebate":"0",
		"ExecutionTime":"2023-07-28 00:13:15.463 +0800 CST",
		"BlockTradeId": "",
	}
	*/
	timeNow := time.Now()
	trade := v3.Trade{
		Symbol:        "DOTUSDT",
		Id:            "1474200510090276864",
		OrderId:       "1474200270671015936",
		TradeId:       "2100000000031181772",
		OrderPrice:    fixedpoint.NewFromFloat(27628),
		OrderQty:      fixedpoint.NewFromFloat(0.007959),
		ExecFee:       fixedpoint.NewFromFloat(0.21989125),
		FeeTokenId:    "USDT",
		CreatTime:     types.MillisecondTimestamp(timeNow),
		IsBuyer:       "0",
		IsMaker:       "0",
		MatchOrderId:  "5760912963729109504",
		MakerRebate:   fixedpoint.NewFromFloat(0),
		ExecutionTime: types.MillisecondTimestamp(timeNow),
		BlockTradeId:  "",
	}

	s, err := toV3Buyer(trade.IsBuyer)
	assert.NoError(t, err)
	m, err := toV3Maker(trade.IsMaker)
	assert.NoError(t, err)
	orderIdNum, err := strconv.ParseUint(trade.OrderId, 10, 64)
	assert.NoError(t, err)
	tradeId, err := strconv.ParseUint(trade.TradeId, 10, 64)
	assert.NoError(t, err)

	exp := types.Trade{
		ID:            tradeId,
		OrderID:       orderIdNum,
		Exchange:      types.ExchangeBybit,
		Price:         trade.OrderPrice,
		Quantity:      trade.OrderQty,
		QuoteQuantity: trade.OrderPrice.Mul(trade.OrderQty),
		Symbol:        trade.Symbol,
		Side:          s,
		IsBuyer:       s == types.SideTypeBuy,
		IsMaker:       m,
		Time:          types.Time(timeNow),
		Fee:           trade.ExecFee,
		FeeCurrency:   trade.FeeTokenId,
		IsMargin:      false,
		IsFutures:     false,
		IsIsolated:    false,
	}
	res, err := v3ToGlobalTrade(trade)
	assert.NoError(t, err)
	assert.Equal(t, res, &exp)
}

func Test_toGlobalKLines(t *testing.T) {
	symbol := "BTCUSDT"
	interval := types.Interval15m

	resp := bybitapi.KLinesResponse{
		Symbol: symbol,
		List: []bybitapi.KLine{
			/*
				[
					{
						"StartTime": "2023-08-08 17:30:00 +0800 CST",
						"OpenPrice": 29045.3,
						"HighPrice": 29228.56,
						"LowPrice": 29045.3,
						"ClosePrice": 29228.56,
						"Volume": 9.265593,
						"TurnOver": 270447.43520753
					},
					{
						"StartTime": "2023-08-08 17:15:00 +0800 CST",
						"OpenPrice": 29167.33,
						"HighPrice": 29229.08,
						"LowPrice": 29000,
						"ClosePrice": 29045.3,
						"Volume": 9.295508,
						"TurnOver": 270816.87513775
					}
				]
			*/
			{
				StartTime: types.NewMillisecondTimestampFromInt(1691486100000),
				Open:      fixedpoint.NewFromFloat(29045.3),
				High:      fixedpoint.NewFromFloat(29228.56),
				Low:       fixedpoint.NewFromFloat(29045.3),
				Close:     fixedpoint.NewFromFloat(29228.56),
				Volume:    fixedpoint.NewFromFloat(9.265593),
				TurnOver:  fixedpoint.NewFromFloat(270447.43520753),
			},
			{
				StartTime: types.NewMillisecondTimestampFromInt(1691487000000),
				Open:      fixedpoint.NewFromFloat(29167.33),
				High:      fixedpoint.NewFromFloat(29229.08),
				Low:       fixedpoint.NewFromFloat(29000),
				Close:     fixedpoint.NewFromFloat(29045.3),
				Volume:    fixedpoint.NewFromFloat(9.295508),
				TurnOver:  fixedpoint.NewFromFloat(270816.87513775),
			},
		},
		Category: bybitapi.CategorySpot,
	}

	expKlines := []types.KLine{
		{
			Exchange:    types.ExchangeBybit,
			Symbol:      resp.Symbol,
			StartTime:   types.Time(resp.List[0].StartTime.Time()),
			EndTime:     types.Time(resp.List[0].StartTime.Time().Add(interval.Duration())),
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
			Exchange:    types.ExchangeBybit,
			Symbol:      resp.Symbol,
			StartTime:   types.Time(resp.List[1].StartTime.Time()),
			EndTime:     types.Time(resp.List[1].StartTime.Time().Add(interval.Duration())),
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

	assert.Equal(t, toGlobalKLines(symbol, interval, resp.List), expKlines)
}

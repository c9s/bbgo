package binance

import (
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var jsCommentTrimmer = regexp.MustCompile("(?m)//.*$")

func TestMarginResponseParsing(t *testing.T) {
	type testcase struct {
		input string
	}

	var testcases = []testcase{
		{
			input: `{
			  "e": "executionReport",
			  "E": 1608545107403,
			  "s": "BNBUSDT",
			  "c": "ios_0de017ca7ceb4102b3baa664feb46e65",
			  "S": "BUY",
			  "o": "MARKET",
			  "f": "GTC",
			  "q": "0.50000000",
			  "p": "0.00000000",
			  "P": "0.00000000",
			  "F": "0.00000000",
			  "g": -1,
			  "C": "",
			  "x": "NEW",
			  "X": "NEW",
			  "r": "NONE",
			  "i": 1147335332,
			  "l": "0.00000000",
			  "z": "0.00000000",
			  "L": "0.00000000",
			  "n": "0",
			  "N": null,
			  "T": 1608545107401,
			  "t": -1,
			  "I": 2387303818,
			  "w": true,
			  "m": false,
			  "M": false,
			  "O": 1608545107401,
			  "Z": "0.00000000",
			  "Y": "0.00000000",
			  "Q": "0.00000000"
			}`,
		},
		{
			input: `{
			  "e": "executionReport",
			  "E": 1608545107403,
			  "s": "BNBUSDT",
			  "c": "ios_0de017ca7ceb4102b3baa664feb46e65",
			  "S": "BUY",
			  "o": "MARKET",
			  "f": "GTC",
			  "q": "0.50000000",
			  "p": "0.00000000",
			  "P": "0.00000000",
			  "F": "0.00000000",
			  "g": -1,
			  "C": "",
			  "x": "TRADE",
			  "X": "FILLED",
			  "r": "NONE",
			  "i": 1147335332,
			  "l": "0.50000000",
			  "z": "0.50000000",
			  "L": "33.85710000",
			  "n": "0.00037500",
			  "N": "BNB",
			  "T": 1608545107401,
			  "t": 98414801,
			  "I": 2387303819,
			  "w": false,
			  "m": false,
			  "M": true,
			  "O": 1608545107401,
			  "Z": "16.92855000",
			  "Y": "16.92855000",
			  "Q": "0.00000000"
			}`,
		},
		{
			input: `{
			  "e": "outboundAccountInfo",
			  "E": 1608545107403,
			  "m": 9,
			  "t": 10,
			  "b": 0,
			  "s": 0,
			  "T": true,
			  "W": true,
			  "D": true,
			  "u": 1608545107401,
			  "B": [
				{
				  "a": "BNB",
				  "f": "89.99345616",
				  "l": "12.00000000"
				},
				{
				  "a": "USDT",
				  "f": "0.16410063",
				  "l": "0.00000000"
				}
			  ],
			  "P": []
			}`,
		},
		{
			input: `{
				"e":"outboundAccountPosition",
				"E":1608545107403,
				"u":1608545107401,
				"B":[{"a":"BNB","f":"89.99345616","l":"12.00000000"},{"a":"USDT","f":"0.16410063","l":"0.00000000"}]
			}`,
		},
	}

	for _, testcase := range testcases {
		payload := testcase.input
		payload = jsCommentTrimmer.ReplaceAllLiteralString(payload, "")
		event, err := parseWebSocketEvent([]byte(payload))
		assert.NoError(t, err)
		assert.NotNil(t, event)
	}
}

func TestParseOrderUpdate(t *testing.T) {
	payload := `{
  "e": "executionReport",        // Event type
  "E": 1499405658658,            // Event time
  "s": "ETHBTC",                 // Symbol
  "c": "mUvoqJxFIILMdfAW5iGSOW", // Client order ID
  "S": "BUY",                    // Side
  "o": "LIMIT",                  // Order type
  "f": "GTC",                    // Time in force
  "q": "1.00000000",             // Order quantity
  "p": "0.10264410",             // Order price
  "P": "0.222",                  // Stop price
  "F": "0.00000000",             // Iceberg quantity
  "g": -1,                       // OrderListId
  "C": null,                     // Original client order ID; This is the ID of the order being canceled
  "x": "NEW",                    // Current execution type
  "X": "NEW",                    // Current order status
  "r": "NONE",                   // Order reject reason; will be an error code.
  "i": 4293153,                  // Order ID
  "l": "0.00000000",             // Last executed quantity
  "z": "0.00000000",             // Cumulative filled quantity
  "L": "0.00000001",             // Last executed price
  "n": "0",                      // Commission amount
  "N": null,                     // Commission asset
  "T": 1499405658657,            // Transaction time
  "t": -1,                       // Trade ID
  "I": 8641984,                  // Ignore
  "w": true,                     // Is the order on the book?
  "m": false,                    // Is this trade the maker side?
  "M": true,                     // Ignore
  "O": 1499405658657,            // Order creation time
  "Z": "0.1",                    // Cumulative quote asset transacted quantity
  "Y": "0.00000000",             // Last quote asset transacted quantity (i.e. lastPrice * lastQty)
  "Q": "2.0"                     // Quote Order Quantity
}`

	payload = jsCommentTrimmer.ReplaceAllLiteralString(payload, "")

	event, err := parseWebSocketEvent([]byte(payload))
	assert.NoError(t, err)
	assert.NotNil(t, event)

	executionReport, ok := event.(*ExecutionReportEvent)
	assert.True(t, ok)
	assert.NotNil(t, executionReport)

	assert.Equal(t, executionReport.Symbol, "ETHBTC")
	assert.Equal(t, executionReport.Side, "BUY")
	assert.Equal(t, executionReport.ClientOrderID, "mUvoqJxFIILMdfAW5iGSOW")
	assert.Equal(t, executionReport.OriginalClientOrderID, "")
	assert.Equal(t, executionReport.OrderType, "LIMIT")
	assert.Equal(t, int64(1499405658657), executionReport.OrderCreationTime.Time().UnixMilli())
	assert.Equal(t, executionReport.TimeInForce, "GTC")
	assert.Equal(t, executionReport.IcebergQuantity, fixedpoint.MustNewFromString("0.00000000"))
	assert.Equal(t, executionReport.OrderQuantity, fixedpoint.MustNewFromString("1.00000000"))
	assert.Equal(t, executionReport.QuoteOrderQuantity, fixedpoint.MustNewFromString("2.0"))
	assert.Equal(t, executionReport.OrderPrice, fixedpoint.MustNewFromString("0.10264410"))
	assert.Equal(t, executionReport.StopPrice, fixedpoint.MustNewFromString("0.222"))
	assert.Equal(t, executionReport.IsOnBook, true)
	assert.Equal(t, executionReport.IsMaker, false)
	assert.Equal(t, executionReport.Ignore, true)
	assert.Equal(t, executionReport.CommissionAmount, fixedpoint.MustNewFromString("0"))
	assert.Equal(t, executionReport.CommissionAsset, "")
	assert.Equal(t, executionReport.CurrentExecutionType, "NEW")
	assert.Equal(t, executionReport.CurrentOrderStatus, "NEW")
	assert.Equal(t, executionReport.OrderID, int64(4293153))
	assert.Equal(t, executionReport.Ignored, int64(8641984))
	assert.Equal(t, executionReport.TradeID, int64(-1))
	assert.Equal(t, int64(1499405658657), executionReport.TransactionTime.Time().UnixMilli())
	assert.Equal(t, fixedpoint.MustNewFromString("0.00000000"), executionReport.LastExecutedQuantity)
	assert.Equal(t, fixedpoint.MustNewFromString("0.00000001"), executionReport.LastExecutedPrice)
	assert.Equal(t, fixedpoint.MustNewFromString("0.00000000"), executionReport.CumulativeFilledQuantity)
	assert.Equal(t, executionReport.CumulativeQuoteAssetTransactedQuantity, fixedpoint.MustNewFromString("0.1"))
	assert.Equal(t, executionReport.LastQuoteAssetTransactedQuantity, fixedpoint.MustNewFromString("0.00000000"))

	orderUpdate, err := executionReport.Order()
	assert.NoError(t, err)
	assert.NotNil(t, orderUpdate)
}

func TestFuturesResponseParsing(t *testing.T) {
	type testcase struct {
		input string
	}

	var testcases = []testcase{
		{
			input: `{
			  "e": "ORDER_TRADE_UPDATE",
			  "T": 1639933384755,
			  "E": 1639933384763,
			  "o": {
				"s": "BTCUSDT",
				"c": "x-NSUYEBKMe60cf610-f5c7-49a4-9c1",
				"S": "SELL",
				"o": "MARKET",
				"f": "GTC",
				"q": "0.001",
				"p": "0",
				"ap": "0",
				"sp": "0",
				"x": "NEW",
				"X": "NEW",
				"i": 38541728873,
				"l": "0",
				"z": "0",
				"L": "0",
				"T": 1639933384755,
				"t": 0,
				"b": "0",
				"a": "0",
				"m": false,
				"R": false,
				"wt": "CONTRACT_PRICE",
				"ot": "MARKET",
				"ps": "BOTH",
				"cp": false,
				"rp": "0",
				"pP": false,
				"si": 0,
				"ss": 0
			  }
			}`,
		},
		{
			input: `{
			  "e": "ACCOUNT_UPDATE",
			  "T": 1639933384755,
			  "E": 1639933384763,
			  "a": {
				"B": [
				  {
					"a": "USDT",
					"wb": "86.94966888",
					"cw": "86.94966888",
					"bc": "0"
				  }
				],
				"P": [
				  {
					"s": "BTCUSDT",
					"pa": "-0.001",
					"ep": "47202.40000",
					"cr": "7.78107001",
					"up": "-0.00233523",
					"mt": "cross",
					"iw": "0",
					"ps": "BOTH",
					"ma": "USDT"
				  }
				],
				"m": "ORDER"
			  }
			}`,
		},
		{
			input: `{
			  "e": "ORDER_TRADE_UPDATE",
			  "T": 1639933384755,
			  "E": 1639933384763,
			  "o": {
				"s": "BTCUSDT",
				"c": "x-NSUYEBKMe60cf610-f5c7-49a4-9c1",
				"S": "SELL",
				"o": "MARKET",
				"f": "GTC",
				"q": "0.001",
				"p": "0",
				"ap": "47202.40000",
				"sp": "0",
				"x": "TRADE",
				"X": "FILLED",
				"i": 38541728873,
				"l": "0.001",
				"z": "0.001",
				"L": "47202.40",
				"n": "0.01888095",
				"N": "USDT",
				"T": 1639933384755,
				"t": 1741505949,
				"b": "0",
				"a": "0",
				"m": false,
				"R": false,
				"wt": "CONTRACT_PRICE",
				"ot": "MARKET",
				"ps": "BOTH",
				"cp": false,
				"rp": "0",
				"pP": false,
				"si": 0,
				"ss": 0
			  }
			}`,
		},
		{
			input: `{
				"e": "TRADE_LITE",
				"E": 1721895408092,
				"T": 1721895408214,
				"s": "BTCUSDT",
				"q": "0.001",
				"p": "0",
				"m": false,
				"c": "z8hcUoOsqEdKMeKPSABslD",
				"S": "BUY",
				"L": "64089.20",
				"l": "0.040",
				"t": 109100866,
				"i": 8886774
			}`,
		},
	}

	for _, testcase := range testcases {
		payload := testcase.input
		payload = jsCommentTrimmer.ReplaceAllLiteralString(payload, "")
		event, err := parseWebSocketEvent([]byte(payload))
		assert.NoError(t, err)
		assert.NotNil(t, event)
	}
}

func TestParseOrderFuturesUpdate(t *testing.T) {
	payload := `{
				  "e": "ORDER_TRADE_UPDATE",
				  "T": 1639933384755,
				  "E": 1639933384763,
				  "o": {
					"s": "BTCUSDT",
					"c": "x-NSUYEBKMe60cf610-f5c7-49a4-9c1",
					"S": "SELL",
					"o": "MARKET",
					"f": "GTC",
					"q": "0.001",
					"p": "0",
					"ap": "47202.40000",
					"sp": "0",
					"x": "TRADE",
					"X": "FILLED",
					"i": 38541728873,
					"l": "0.001",
					"z": "0.001",
					"L": "47202.40",
					"n": "0.01888095",
					"N": "USDT",
					"T": 1639933384755,
					"t": 1741505949,
					"b": "0",
					"a": "0",
					"m": false,
					"R": false,
					"wt": "CONTRACT_PRICE",
					"ot": "MARKET",
					"ps": "BOTH",
					"cp": false,
					"rp": "0",
					"pP": false,
					"si": 0,
					"ss": 0
				  }
				}`

	payload = jsCommentTrimmer.ReplaceAllLiteralString(payload, "")

	event, err := parseWebSocketEvent([]byte(payload))
	assert.NoError(t, err)
	assert.NotNil(t, event)

	orderTradeEvent, ok := event.(*OrderTradeUpdateEvent)
	assert.True(t, ok)
	assert.NotNil(t, orderTradeEvent)

	assert.Equal(t, "BTCUSDT", orderTradeEvent.OrderTrade.Symbol)
	assert.Equal(t, "SELL", orderTradeEvent.OrderTrade.Side)
	assert.Equal(t, "x-NSUYEBKMe60cf610-f5c7-49a4-9c1", orderTradeEvent.OrderTrade.ClientOrderID)
	assert.Equal(t, "MARKET", orderTradeEvent.OrderTrade.OrderType)
	assert.Equal(t, types.NewMillisecondTimestampFromInt(1639933384763), orderTradeEvent.Time)
	assert.Equal(t, types.MillisecondTimestamp(time.UnixMilli(1639933384755)), orderTradeEvent.OrderTrade.OrderTradeTime)
	assert.Equal(t, fixedpoint.MustNewFromString("0.001"), orderTradeEvent.OrderTrade.OriginalQuantity)
	assert.Equal(t, fixedpoint.MustNewFromString("0.001"), orderTradeEvent.OrderTrade.OrderLastFilledQuantity)
	assert.Equal(t, fixedpoint.MustNewFromString("0.001"), orderTradeEvent.OrderTrade.OrderFilledAccumulatedQuantity)
	assert.Equal(t, "TRADE", orderTradeEvent.OrderTrade.CurrentExecutionType)
	assert.Equal(t, "FILLED", orderTradeEvent.OrderTrade.CurrentOrderStatus)
	assert.Equal(t, fixedpoint.MustNewFromString("47202.40"), orderTradeEvent.OrderTrade.LastFilledPrice)
	assert.Equal(t, int64(38541728873), orderTradeEvent.OrderTrade.OrderId)
	assert.Equal(t, int64(1741505949), orderTradeEvent.OrderTrade.TradeId)

	orderUpdate, err := orderTradeEvent.OrderFutures()
	assert.NoError(t, err)
	assert.NotNil(t, orderUpdate)
}

func TestAlgoOrderUpdateEvent_OrderFutures(t *testing.T) {
	transactionTime := types.MillisecondTimestamp(time.UnixMilli(1750515742297))

	t.Run("valid status NEW", func(t *testing.T) {
		event := &AlgoOrderUpdateEvent{
			TransactionTime: transactionTime,
			AlgoOrder: AlgoOrder{
				ClientAlgoId:     "Q5xaq5EGKgXXa0fD7fs0Ip",
				AlgoId:           2148719,
				AlgoType:         "CONDITIONAL",
				OrderType:        "TAKE_PROFIT",
				Symbol:           "BNBUSDT",
				Side:             "SELL",
				PositionSide:     "BOTH",
				TimeInForce:      "GTC",
				Quantity:         fixedpoint.MustNewFromString("0.01"),
				Price:            fixedpoint.MustNewFromString("750"),
				TriggerPrice:     fixedpoint.MustNewFromString("750"),
				Status:           "NEW",
				OrderId:          "",
				AvgFillPrice:     fixedpoint.MustNewFromString("0.00000"),
				ExecutedQuantity: fixedpoint.MustNewFromString("0.00000"),
				ActualOrderType:  "0",
				STPMode:          "EXPIRE_MAKER",
				WorkingType:      "CONTRACT_PRICE",
				PriceMatchMode:   "NONE",
				CloseAll:         false,
				PriceProtection:  false,
				TriggerTime:      0,
				GoodTillTime:     0,
				ReduceOnly:       false,
				FailedReason:     "",
			},
		}

		order, err := event.OrderFutures()
		assert.NoError(t, err)
		assert.NotNil(t, order)
		assert.Equal(t, types.ExchangeBinance, order.Exchange)
		assert.Equal(t, "BNBUSDT", order.Symbol)
		assert.Equal(t, "Q5xaq5EGKgXXa0fD7fs0Ip", order.ClientOrderID)
		assert.Equal(t, types.SideTypeSell, order.Side)
		assert.Equal(t, types.OrderTypeTakeProfit, order.Type)
		assert.Equal(t, fixedpoint.MustNewFromString("0.01"), order.Quantity)
		assert.Equal(t, fixedpoint.MustNewFromString("750"), order.Price)
		assert.Equal(t, fixedpoint.MustNewFromString("750"), order.StopPrice)
		assert.Equal(t, types.TimeInForce("GTC"), order.TimeInForce)
		assert.Equal(t, uint64(2148719), order.OrderID)
		assert.Equal(t, types.OrderStatusNew, order.Status)
		assert.Equal(t, fixedpoint.MustNewFromString("0.00000"), order.ExecutedQuantity)
		assert.Equal(t, transactionTime.Time(), order.UpdateTime.Time())
	})

	t.Run("valid status CANCELED", func(t *testing.T) {
		event := &AlgoOrderUpdateEvent{
			TransactionTime: transactionTime,
			AlgoOrder: AlgoOrder{
				ClientAlgoId:     "test-client-id",
				AlgoId:           123456,
				OrderType:        "STOP",
				Symbol:           "BTCUSDT",
				Side:             "BUY",
				TimeInForce:      "GTC",
				Quantity:         fixedpoint.MustNewFromString("1.0"),
				Price:            fixedpoint.MustNewFromString("50000"),
				TriggerPrice:     fixedpoint.MustNewFromString("49000"),
				Status:           "CANCELED",
				ExecutedQuantity: fixedpoint.MustNewFromString("0"),
			},
		}

		order, err := event.OrderFutures()
		assert.NoError(t, err)
		assert.NotNil(t, order)
		assert.Equal(t, types.OrderStatusCanceled, order.Status)
	})

	t.Run("valid status EXPIRED", func(t *testing.T) {
		event := &AlgoOrderUpdateEvent{
			TransactionTime: transactionTime,
			AlgoOrder: AlgoOrder{
				ClientAlgoId:     "test-expired",
				AlgoId:           789012,
				OrderType:        "TAKE_PROFIT_MARKET",
				Symbol:           "ETHUSDT",
				Side:             "SELL",
				TimeInForce:      "GTC",
				Quantity:         fixedpoint.MustNewFromString("5.0"),
				Price:            fixedpoint.MustNewFromString("3000"),
				TriggerPrice:     fixedpoint.MustNewFromString("3200"),
				Status:           "EXPIRED",
				ExecutedQuantity: fixedpoint.MustNewFromString("0"),
			},
		}

		order, err := event.OrderFutures()
		assert.NoError(t, err)
		assert.NotNil(t, order)
		assert.Equal(t, types.OrderStatusExpired, order.Status)
	})

	t.Run("valid status REJECTED", func(t *testing.T) {
		event := &AlgoOrderUpdateEvent{
			TransactionTime: transactionTime,
			AlgoOrder: AlgoOrder{
				ClientAlgoId:     "test-rejected",
				AlgoId:           345678,
				OrderType:        "STOP_MARKET",
				Symbol:           "SOLUSDT",
				Side:             "BUY",
				TimeInForce:      "GTC",
				Quantity:         fixedpoint.MustNewFromString("10.0"),
				Price:            fixedpoint.MustNewFromString("100"),
				TriggerPrice:     fixedpoint.MustNewFromString("95"),
				Status:           "REJECTED",
				FailedReason:     "Insufficient balance",
				ExecutedQuantity: fixedpoint.MustNewFromString("0"),
			},
		}

		order, err := event.OrderFutures()
		assert.NoError(t, err)
		assert.NotNil(t, order)
		assert.Equal(t, types.OrderStatusRejected, order.Status)
	})

	t.Run("valid status TRIGGERING", func(t *testing.T) {
		event := &AlgoOrderUpdateEvent{
			TransactionTime: transactionTime,
			AlgoOrder: AlgoOrder{
				ClientAlgoId:     "test-triggering",
				AlgoId:           456789,
				OrderType:        "STOP",
				Symbol:           "ADAUSDT",
				Side:             "SELL",
				TimeInForce:      "GTC",
				Quantity:         fixedpoint.MustNewFromString("1000.0"),
				Price:            fixedpoint.MustNewFromString("0.5"),
				TriggerPrice:     fixedpoint.MustNewFromString("0.48"),
				Status:           "TRIGGERING",
				ExecutedQuantity: fixedpoint.MustNewFromString("0"),
			},
		}

		order, err := event.OrderFutures()
		assert.NoError(t, err)
		assert.NotNil(t, order)
		assert.Equal(t, uint64(456789), order.OrderID)
	})

	t.Run("valid status TRIGGERED", func(t *testing.T) {
		event := &AlgoOrderUpdateEvent{
			TransactionTime: transactionTime,
			AlgoOrder: AlgoOrder{
				ClientAlgoId:     "test-triggered",
				AlgoId:           567890,
				OrderType:        "TAKE_PROFIT_MARKET",
				Symbol:           "DOGEUSDT",
				Side:             "BUY",
				TimeInForce:      "GTC",
				Quantity:         fixedpoint.MustNewFromString("10000.0"),
				Price:            fixedpoint.MustNewFromString("0.1"),
				TriggerPrice:     fixedpoint.MustNewFromString("0.12"),
				Status:           "TRIGGERED",
				OrderId:          "12345",
				AvgFillPrice:     fixedpoint.MustNewFromString("0.11"),
				ExecutedQuantity: fixedpoint.MustNewFromString("5000.0"),
				ActualOrderType:  "MARKET",
			},
		}

		order, err := event.OrderFutures()
		assert.NoError(t, err)
		assert.NotNil(t, order)
		assert.Equal(t, fixedpoint.MustNewFromString("5000.0"), order.ExecutedQuantity)
	})

	t.Run("valid status FINISHED", func(t *testing.T) {
		event := &AlgoOrderUpdateEvent{
			TransactionTime: transactionTime,
			AlgoOrder: AlgoOrder{
				ClientAlgoId:     "test-finished",
				AlgoId:           678901,
				OrderType:        "TRAILING_STOP_MARKET",
				Symbol:           "XRPUSDT",
				Side:             "SELL",
				TimeInForce:      "GTC",
				Quantity:         fixedpoint.MustNewFromString("500.0"),
				Price:            fixedpoint.MustNewFromString("0.6"),
				TriggerPrice:     fixedpoint.MustNewFromString("0.58"),
				Status:           "FINISHED",
				OrderId:          "67890",
				AvgFillPrice:     fixedpoint.MustNewFromString("0.59"),
				ExecutedQuantity: fixedpoint.MustNewFromString("500.0"),
				ActualOrderType:  "MARKET",
			},
		}

		order, err := event.OrderFutures()
		assert.NoError(t, err)
		assert.NotNil(t, order)
		assert.Equal(t, fixedpoint.MustNewFromString("500.0"), order.ExecutedQuantity)
	})

	t.Run("invalid status returns error", func(t *testing.T) {
		event := &AlgoOrderUpdateEvent{
			TransactionTime: transactionTime,
			AlgoOrder: AlgoOrder{
				ClientAlgoId: "test-invalid",
				AlgoId:       999999,
				OrderType:    "STOP",
				Symbol:       "BTCUSDT",
				Side:         "BUY",
				Status:       "INVALID_STATUS",
			},
		}

		order, err := event.OrderFutures()
		assert.Error(t, err)
		assert.Nil(t, order)
		assert.Contains(t, err.Error(), "algo update event type is not for futures order")
	})

	t.Run("with executed quantity and avg fill price", func(t *testing.T) {
		event := &AlgoOrderUpdateEvent{
			TransactionTime: transactionTime,
			AlgoOrder: AlgoOrder{
				ClientAlgoId:     "test-executed",
				AlgoId:           111222,
				OrderType:        "TAKE_PROFIT",
				Symbol:           "LTCUSDT",
				Side:             "BUY",
				TimeInForce:      "GTC",
				Quantity:         fixedpoint.MustNewFromString("2.0"),
				Price:            fixedpoint.MustNewFromString("100"),
				TriggerPrice:     fixedpoint.MustNewFromString("95"),
				Status:           "TRIGGERED",
				OrderId:          "99999",
				AvgFillPrice:     fixedpoint.MustNewFromString("97.5"),
				ExecutedQuantity: fixedpoint.MustNewFromString("1.5"),
				ActualOrderType:  "LIMIT",
			},
		}

		order, err := event.OrderFutures()
		assert.NoError(t, err)
		assert.NotNil(t, order)
		assert.Equal(t, fixedpoint.MustNewFromString("1.5"), order.ExecutedQuantity)
	})

	t.Run("different order types", func(t *testing.T) {
		testCases := []struct {
			name      string
			orderType string
			expected  types.OrderType
		}{
			{"STOP", "STOP", types.OrderTypeStopLimit},
			{"STOP_MARKET", "STOP_MARKET", types.OrderTypeStopMarket},
			{"TAKE_PROFIT", "TAKE_PROFIT", types.OrderTypeTakeProfit},
			{"TAKE_PROFIT_MARKET", "TAKE_PROFIT_MARKET", types.OrderTypeTakeProfitMarket},
			{"TRAILING_STOP_MARKET", "TRAILING_STOP_MARKET", types.OrderTypeStopMarket},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				event := &AlgoOrderUpdateEvent{
					TransactionTime: transactionTime,
					AlgoOrder: AlgoOrder{
						ClientAlgoId:     "test-" + tc.name,
						AlgoId:           111111,
						OrderType:        tc.orderType,
						Symbol:           "BTCUSDT",
						Side:             "BUY",
						TimeInForce:      "GTC",
						Quantity:         fixedpoint.MustNewFromString("1.0"),
						Price:            fixedpoint.MustNewFromString("50000"),
						TriggerPrice:     fixedpoint.MustNewFromString("49000"),
						Status:           "NEW",
						ExecutedQuantity: fixedpoint.MustNewFromString("0"),
					},
				}

				order, err := event.OrderFutures()
				assert.NoError(t, err)
				assert.NotNil(t, order)
				assert.Equal(t, tc.expected, order.Type)
			})
		}
	})

	t.Run("different sides", func(t *testing.T) {
		testCases := []struct {
			name     string
			side     string
			expected types.SideType
		}{
			{"BUY", "BUY", types.SideTypeBuy},
			{"SELL", "SELL", types.SideTypeSell},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				event := &AlgoOrderUpdateEvent{
					TransactionTime: transactionTime,
					AlgoOrder: AlgoOrder{
						ClientAlgoId:     "test-side-" + tc.name,
						AlgoId:           222222,
						OrderType:        "STOP",
						Symbol:           "BTCUSDT",
						Side:             tc.side,
						TimeInForce:      "GTC",
						Quantity:         fixedpoint.MustNewFromString("1.0"),
						Price:            fixedpoint.MustNewFromString("50000"),
						TriggerPrice:     fixedpoint.MustNewFromString("49000"),
						Status:           "NEW",
						ExecutedQuantity: fixedpoint.MustNewFromString("0"),
					},
				}

				order, err := event.OrderFutures()
				assert.NoError(t, err)
				assert.NotNil(t, order)
				assert.Equal(t, tc.expected, order.Side)
			})
		}
	})
}

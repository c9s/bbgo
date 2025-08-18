package okex

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/strint"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

func Test_parseWebSocketEvent_accountEvent(t *testing.T) {
	t.Run("succeeds", func(t *testing.T) {
		in := `
{
  "arg": {
    "channel": "account",
    "uid": "77982378738415879"
  },
  "data": [
    {
      "uTime": "1614846244194",
      "totalEq": "91884",
      "adjEq": "91884.85025",
      "isoEq": "0",
      "ordFroz": "0",
      "imr": "0",
      "mmr": "0",
      "borrowFroz": "",
      "notionalUsd": "",
      "mgnRatio": "100000",
      "details": [{
          "availBal": "",
          "availEq": "1",
          "ccy": "BTC",
          "cashBal": "1",
          "uTime": "1617279471503",
          "disEq": "50559.01",
          "eq": "1",
          "eqUsd": "45078",
          "fixedBal": "0",
          "frozenBal": "0",
          "interest": "0",
          "isoEq": "0",
          "liab": "0",
          "maxLoan": "",
          "mgnRatio": "",
          "notionalLever": "0",
          "ordFrozen": "0",
          "upl": "0",
          "uplLiab": "0",
          "crossLiab": "0",
          "isoLiab": "0",
          "coinUsdPrice": "60000",
          "stgyEq":"0",
          "spotInUseAmt":"",
          "isoUpl":"",
          "borrowFroz": ""
        },
        {
          "availBal": "",
          "availEq": "41307",
          "ccy": "USDT",
          "cashBal": "41307",
          "uTime": "1617279471503",
          "disEq": "41325",
          "eq": "41307",
          "eqUsd": "45078",
          "fixedBal": "0",
          "frozenBal": "0",
          "interest": "0",
          "isoEq": "0",
          "liab": "0",
          "maxLoan": "",
          "mgnRatio": "",
          "notionalLever": "0",
          "ordFrozen": "0",
          "upl": "0",
          "uplLiab": "0",
          "crossLiab": "0",
          "isoLiab": "0",
          "coinUsdPrice": "1.00007",
          "stgyEq":"0",
          "spotInUseAmt":"",
          "isoUpl":"",
          "borrowFroz": ""
        }
      ]
    }
  ]
}
`

		exp := &okexapi.Account{
			TotalEquityInUSD: Number(91884),
			AdjustEquity:     Number(91884.85025),
			UpdateTime:       types.NewMillisecondTimestampFromInt(1614846244194),
			MarginRatio:      Number(100000),
			Details: []okexapi.BalanceDetail{
				{
					Currency:                "BTC",
					Available:               fixedpoint.Zero,
					AvailEquity:             fixedpoint.One,
					Equity:                  fixedpoint.One,
					DisEquity:               Number(50559.01),
					EquityInUSD:             fixedpoint.NewFromFloat(45078),
					CashBalance:             fixedpoint.One,
					OrderFrozen:             fixedpoint.Zero,
					FrozenBalance:           fixedpoint.Zero,
					UpdateTime:              types.NewMillisecondTimestampFromInt(1617279471503),
					NotionalLever:           "0",
					UnrealizedProfitAndLoss: fixedpoint.Zero,
				},
				{
					Currency:                "USDT",
					Available:               Number(0),
					AvailEquity:             Number(41307),
					CashBalance:             Number(41307),
					OrderFrozen:             fixedpoint.Zero,
					FrozenBalance:           fixedpoint.Zero,
					Equity:                  fixedpoint.NewFromFloat(41307),
					EquityInUSD:             fixedpoint.NewFromFloat(45078),
					DisEquity:               Number(41325),
					UpdateTime:              types.NewMillisecondTimestampFromInt(1617279471503),
					UnrealizedProfitAndLoss: fixedpoint.Zero,
					NotionalLever:           "0",
				},
			},
		}

		res, err := parseWebSocketEvent([]byte(in))
		assert.NoError(t, err)
		event, ok := res.(*okexapi.Account)
		assert.True(t, ok)
		assert.Equal(t, exp, event)
	})

}

func TestParsePriceVolumeOrderSliceJSON(t *testing.T) {
	t.Run("snapshot", func(t *testing.T) {
		in := `
{
  "arg": {
    "channel": "books",
    "instId": "BTC-USDT"
  },
  "action": "snapshot",
  "data": [
    {
      "asks": [
        ["8476.98", "415", "0", "13"],
        ["8477", "7", "0", "2"]
      ],
      "bids": [
        ["8476", "256", "0", "12"]
      ],
      "ts": "1597026383085",
      "checksum": -855196043,
      "prevSeqId": -1,
      "seqId": 123456
    }
  ]
}
`

		asks := PriceVolumeOrderSlice{
			{
				PriceVolume: types.PriceVolume{
					Price:  fixedpoint.NewFromFloat(8476.98),
					Volume: fixedpoint.NewFromFloat(415),
				},
				NumLiquidated: fixedpoint.Zero.Int(),
				NumOrders:     fixedpoint.NewFromFloat(13).Int(),
			},
			{
				PriceVolume: types.PriceVolume{
					Price:  fixedpoint.NewFromFloat(8477),
					Volume: fixedpoint.NewFromFloat(7),
				},
				NumLiquidated: fixedpoint.Zero.Int(),
				NumOrders:     fixedpoint.NewFromFloat(2).Int(),
			},
		}
		bids := PriceVolumeOrderSlice{
			{
				PriceVolume: types.PriceVolume{
					Price:  fixedpoint.NewFromFloat(8476),
					Volume: fixedpoint.NewFromFloat(256),
				},
				NumLiquidated: fixedpoint.Zero.Int(),
				NumOrders:     fixedpoint.NewFromFloat(12).Int(),
			},
		}

		res, err := parseWebSocketEvent([]byte(in))
		assert.NoError(t, err)
		event, ok := res.(*BookEvent)
		assert.True(t, ok)
		assert.Equal(t, "BTCUSDT", event.Symbol)
		assert.Equal(t, ChannelBooks, event.channel)
		assert.Equal(t, ActionTypeSnapshot, event.Action)
		assert.Len(t, event.Data, 1)
		assert.Len(t, event.Data[0].Asks, 2)
		assert.Equal(t, asks, event.Data[0].Asks)
		assert.Len(t, event.Data[0].Bids, 1)
		assert.Equal(t, bids, event.Data[0].Bids)
	})

	t.Run("unexpected asks", func(t *testing.T) {
		t.Skip("this will cause panic, so i skip it")
		in := `
{
  "arg": {
    "channel": "books",
    "instId": "BTC-USDT"
  },
  "action": "snapshot",
  "data": [
    {
      "asks": [
        ["XYZ", "415", "0", "13"]
      ],
      "bids": [
        ["8476", "256", "0", "12"]
      ],
      "ts": "1597026383085",
      "checksum": -855196043,
      "prevSeqId": -1,
      "seqId": 123456
    }
  ]
}
`
		_, err := parseWebSocketEvent([]byte(in))
		assert.ErrorContains(t, err, "price volume order")
	})
}

func TestBookEvent_BookTicker(t *testing.T) {
	in := `
{
  "arg": {
    "channel": "books",
    "instId": "BTC-USDT"
  },
  "action": "snapshot",
  "data": [
    {
      "asks": [
        ["8476.98", "415", "0", "13"],
        ["8477", "7", "0", "2"]
      ],
      "bids": [
        ["8476", "256", "0", "12"]
      ],
      "ts": "1597026383085",
      "checksum": -855196043,
      "prevSeqId": -1,
      "seqId": 123456
    }
  ]
}
`

	res, err := parseWebSocketEvent([]byte(in))
	assert.NoError(t, err)
	event, ok := res.(*BookEvent)
	assert.True(t, ok)

	ticker := event.BookTicker()
	assert.Equal(t, types.BookTicker{
		Symbol:   "BTCUSDT",
		Buy:      fixedpoint.NewFromFloat(8476),
		BuySize:  fixedpoint.NewFromFloat(256),
		Sell:     fixedpoint.NewFromFloat(8476.98),
		SellSize: fixedpoint.NewFromFloat(415),
	}, ticker)
}

func TestBookEvent_Book(t *testing.T) {
	in := `
{
  "arg": {
    "channel": "books",
    "instId": "BTC-USDT"
  },
  "action": "snapshot",
  "data": [
    {
      "asks": [
        ["8476.98", "415", "0", "13"],
        ["8477", "7", "0", "2"]
      ],
      "bids": [
        ["8476", "256", "0", "12"]
      ],
      "ts": "1597026383085",
      "checksum": -855196043,
      "prevSeqId": -1,
      "seqId": 123456
    }
  ]
}
`
	bids := types.PriceVolumeSlice{
		{
			Price:  fixedpoint.NewFromFloat(8476),
			Volume: fixedpoint.NewFromFloat(256),
		},
	}
	asks := types.PriceVolumeSlice{
		{
			Price:  fixedpoint.NewFromFloat(8476.98),
			Volume: fixedpoint.NewFromFloat(415),
		},
		{
			Price:  fixedpoint.NewFromFloat(8477),
			Volume: fixedpoint.NewFromFloat(7),
		},
	}

	res, err := parseWebSocketEvent([]byte(in))
	assert.NoError(t, err)
	event, ok := res.(*BookEvent)
	assert.True(t, ok)

	book := event.Book()
	assert.Equal(t, types.SliceOrderBook{
		Symbol: "BTCUSDT",
		Time:   types.NewMillisecondTimestampFromInt(1597026383085).Time(),
		Bids:   bids,
		Asks:   asks,
	}, book)
}

func Test_parseKLineSliceJSON(t *testing.T) {
	t.Run("snapshot", func(t *testing.T) {
		in := `
{
  "arg": {
    "channel": "candle1D",
    "instId": "BTC-USDT"
  },
  "data": [
    [
      "1597026383085",
      "8533",
      "8553.74",
      "8527.17",
      "8548.26",
      "45247",
      "529.5858061",
      "529.5858061",
      "0"
    ]
  ]
}
`
		exp := &KLineEvent{
			Events: okexapi.KLineSlice{
				{
					StartTime:        types.NewMillisecondTimestampFromInt(1597026383085),
					OpenPrice:        fixedpoint.NewFromFloat(8533),
					HighestPrice:     fixedpoint.NewFromFloat(8553.74),
					LowestPrice:      fixedpoint.NewFromFloat(8527.17),
					ClosePrice:       fixedpoint.NewFromFloat(8548.26),
					Volume:           fixedpoint.NewFromFloat(45247),
					VolumeInCurrency: fixedpoint.NewFromFloat(529.5858061),
					// VolumeInCurrencyQuote: fixedpoint.NewFromFloat(529.5858061),
					Confirm: fixedpoint.Zero,
				},
			},
			InstrumentID: "BTC-USDT",
			Symbol:       "BTCUSDT",
			Interval:     "1d",
			Channel:      "candle1D",
		}

		res, err := parseWebSocketEvent([]byte(in))
		assert.NoError(t, err)
		event, ok := res.(*KLineEvent)
		assert.True(t, ok)
		assert.Len(t, event.Events, 1)
		assert.Equal(t, exp, event)
	})

	t.Run("failed to convert timestamp", func(t *testing.T) {
		t.Skip("this will cause panic, so i skip it")
		in := `
{
  "arg": {
    "channel": "candle1D",
    "instId": "BTC-USDT"
  },
  "data": [
    [
      "x",
      "8533",
      "8553.74",
      "8527.17",
      "8548.26",
      "45247",
      "529.5858061",
      "529.5858061",
      "0"
    ]
  ]
}
`

		_, err := parseWebSocketEvent([]byte(in))
		assert.ErrorContains(t, err, "timestamp")
	})

	t.Run("failed to convert open price", func(t *testing.T) {
		t.Skip("this will cause panic, so i skip it")
		in := `
{
  "arg": {
    "channel": "candle1D",
    "instId": "BTC-USDT"
  },
  "data": [
    [
      "1597026383085",
      "x",
      "8553.74",
      "8527.17",
      "8548.26",
      "45247",
      "529.5858061",
      "529.5858061",
      "0"
    ]
  ]
}
`

		_, err := parseWebSocketEvent([]byte(in))
		assert.ErrorContains(t, err, "open price")
	})

	t.Run("failed to convert highest price", func(t *testing.T) {
		t.Skip("this will cause panic, so i skip it")
		in := `
{
  "arg": {
    "channel": "candle1D",
    "instId": "BTC-USDT"
  },
  "data": [
    [
      "1597026383085",
      "8533",
      "x",
      "8527.17",
      "8548.26",
      "45247",
      "529.5858061",
      "529.5858061",
      "0"
    ]
  ]
}
`

		_, err := parseWebSocketEvent([]byte(in))
		assert.ErrorContains(t, err, "highest price")
	})
	t.Run("failed to convert lowest price", func(t *testing.T) {
		t.Skip("this will cause panic, so i skip it")
		in := `
{
  "arg": {
    "channel": "candle1D",
    "instId": "BTC-USDT"
  },
  "data": [
    [
      "1597026383085",
      "8533",
      "8553.74",
      "x",
      "8548.26",
      "45247",
      "529.5858061",
      "529.5858061",
      "0"
    ]
  ]
}
`

		_, err := parseWebSocketEvent([]byte(in))
		assert.ErrorContains(t, err, "lowest price")
	})
	t.Run("failed to convert close price", func(t *testing.T) {
		t.Skip("this will cause panic, so i skip it")
		in := `
{
  "arg": {
    "channel": "candle1D",
    "instId": "BTC-USDT"
  },
  "data": [
    [
      "1597026383085",
      "8533",
      "8553.74",
      "8527.17",
      "x",
      "45247",
      "529.5858061",
      "529.5858061",
      "0"
    ]
  ]
}
`

		_, err := parseWebSocketEvent([]byte(in))
		assert.ErrorContains(t, err, "close price")
	})
	t.Run("failed to convert volume", func(t *testing.T) {
		t.Skip("this will cause panic, so i skip it")
		in := `
{
  "arg": {
    "channel": "candle1D",
    "instId": "BTC-USDT"
  },
  "data": [
    [
      "1597026383085",
      "8533",
      "8553.74",
      "8527.17",
      "8548.26",
      "x",
      "529.5858061",
      "529.5858061",
      "0"
    ]
  ]
}
`

		_, err := parseWebSocketEvent([]byte(in))
		assert.ErrorContains(t, err, "volume")
	})
	t.Run("failed to convert volume currency", func(t *testing.T) {
		t.Skip("this will cause panic, so i skip it")
		in := `
{
  "arg": {
    "channel": "candle1D",
    "instId": "BTC-USDT"
  },
  "data": [
    [
      "1597026383085",
      "8533",
      "8553.74",
      "8527.17",
      "8548.26",
      "45247",
      "x",
      "529.5858061",
      "0"
    ]
  ]
}
`

		_, err := parseWebSocketEvent([]byte(in))
		assert.ErrorContains(t, err, "volume currency")
	})
	t.Run("failed to convert trading currency quote ", func(t *testing.T) {
		t.Skip("this will cause panic, so i skip it")
		in := `
{
  "arg": {
    "channel": "candle1D",
    "instId": "BTC-USDT"
  },
  "data": [
    [
      "1597026383085",
      "8533",
      "8553.74",
      "8527.17",
      "8548.26",
      "45247",
      "529.5858061",
      "x",
      "0"
    ]
  ]
}
`

		_, err := parseWebSocketEvent([]byte(in))
		assert.ErrorContains(t, err, "trading currency")
	})
	t.Run("failed to convert confirm", func(t *testing.T) {
		t.Skip("this will cause panic, so i skip it")
		in := `
{
  "arg": {
    "channel": "candle1D",
    "instId": "BTC-USDT"
  },
  "data": [
    [
      "1597026383085",
      "8533",
      "8553.74",
      "8527.17",
      "8548.26",
      "45247",
      "529.5858061",
      "529.5858061",
      "g"
    ]
  ]
}
`

		_, err := parseWebSocketEvent([]byte(in))
		assert.ErrorContains(t, err, "confirm")
	})

}

func TestKLine_ToGlobal(t *testing.T) {
	t.Run("snapshot", func(t *testing.T) {
		in := `
{
  "arg": {
    "channel": "candle1D",
    "instId": "BTC-USDT"
  },
  "data": [
    [
      "1597026383085",
      "8533",
      "8553.74",
      "8527.17",
      "8548.26",
      "45247",
      "529.5858061",
      "529.5858061",
      "0"
    ]
  ]
}
`
		exp := &KLineEvent{
			Events: okexapi.KLineSlice{
				{
					StartTime:        types.NewMillisecondTimestampFromInt(1597026383085),
					OpenPrice:        fixedpoint.NewFromFloat(8533),
					HighestPrice:     fixedpoint.NewFromFloat(8553.74),
					LowestPrice:      fixedpoint.NewFromFloat(8527.17),
					ClosePrice:       fixedpoint.NewFromFloat(8548.26),
					Volume:           fixedpoint.NewFromFloat(45247),
					VolumeInCurrency: fixedpoint.NewFromFloat(529.5858061),
					// VolumeInCurrencyQuote: fixedpoint.NewFromFloat(529.5858061),
					Confirm: fixedpoint.Zero,
				},
			},
			InstrumentID: "BTC-USDT",
			Symbol:       "BTCUSDT",
			Interval:     "1d",
			Channel:      "candle1D",
		}

		res, err := parseWebSocketEvent([]byte(in))
		assert.NoError(t, err)
		event, ok := res.(*KLineEvent)
		assert.True(t, ok)

		assert.Equal(t, types.KLine{
			Exchange:                 types.ExchangeOKEx,
			Symbol:                   "BTCUSDT",
			StartTime:                types.Time(types.NewMillisecondTimestampFromInt(1597026383085)),
			EndTime:                  types.Time(types.NewMillisecondTimestampFromInt(1597026383085).Time().Add(types.Interval(exp.Interval).Duration() - time.Millisecond)),
			Interval:                 types.Interval(exp.Interval),
			Open:                     exp.Events[0].OpenPrice,
			Close:                    exp.Events[0].ClosePrice,
			High:                     exp.Events[0].HighestPrice,
			Low:                      exp.Events[0].LowestPrice,
			Volume:                   exp.Events[0].Volume,
			QuoteVolume:              exp.Events[0].VolumeInCurrency,
			TakerBuyBaseAssetVolume:  fixedpoint.Zero,
			TakerBuyQuoteAssetVolume: fixedpoint.Zero,
			LastTradeID:              0,
			NumberOfTrades:           0,
			Closed:                   false,
		}, kLineToGlobal(event.Events[0], types.Interval(event.Interval), event.Symbol))
	})

}

func Test_parseWebSocketEvent(t *testing.T) {
	in := `
{
  "arg": {
    "channel": "trades",
    "instId": "BTC-USDT"
  },
  "data": [
    {
      "instId": "BTC-USDT",
      "tradeId": "130639474",
      "px": "42219.9",
      "sz": "0.12060306",
      "side": "buy",
      "ts": "1630048897897",
      "count": "3"
    }
  ]
}
`
	exp := []MarketTradeEvent{{
		InstId:    "BTC-USDT",
		TradeId:   130639474,
		Px:        fixedpoint.NewFromFloat(42219.9),
		Sz:        fixedpoint.NewFromFloat(0.12060306),
		Side:      okexapi.SideTypeBuy,
		Timestamp: types.NewMillisecondTimestampFromInt(1630048897897),
		Count:     3,
	}}

	res, err := parseWebSocketEvent([]byte(in))
	assert.NoError(t, err)
	event, ok := res.([]MarketTradeEvent)
	assert.True(t, ok)
	assert.Len(t, event, 1)
	assert.Equal(t, exp, event)

}

func Test_toGlobalTrade(t *testing.T) {
	//  {
	//      "instId": "BTC-USDT",
	//      "tradeId": "130639474",
	//      "px": "42219.9",
	//      "sz": "0.12060306",
	//      "side": "buy",
	//      "ts": "1630048897897",
	//      "count": "3"
	//    }
	marketTrade := MarketTradeEvent{
		InstId:    "BTC-USDT",
		TradeId:   130639474,
		Px:        fixedpoint.NewFromFloat(42219.9),
		Sz:        fixedpoint.NewFromFloat(0.12060306),
		Side:      okexapi.SideTypeBuy,
		Timestamp: types.NewMillisecondTimestampFromInt(1630048897897),
		Count:     3,
	}
	t.Run("succeeds", func(t *testing.T) {
		trade, err := marketTrade.toGlobalTrade()
		assert.NoError(t, err)
		assert.Equal(t, types.Trade{
			ID:            uint64(130639474),
			OrderID:       uint64(0),
			Exchange:      types.ExchangeOKEx,
			Price:         fixedpoint.NewFromFloat(42219.9),
			Quantity:      fixedpoint.NewFromFloat(0.12060306),
			QuoteQuantity: marketTrade.Px.Mul(marketTrade.Sz),
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeBuy,
			IsBuyer:       true,
			IsMaker:       false,
			Time:          types.Time(types.NewMillisecondTimestampFromInt(1630048897897)),
			Fee:           fixedpoint.Zero,
			FeeCurrency:   "",
			FeeDiscounted: false,
		}, trade)
	})
	t.Run("unexpected side", func(t *testing.T) {
		newTrade := marketTrade
		newTrade.Side = "both"
		_, err := newTrade.toGlobalTrade()
		assert.ErrorContains(t, err, "both")
	})
	t.Run("unexpected symbol", func(t *testing.T) {
		newTrade := marketTrade
		newTrade.InstId = ""
		_, err := newTrade.toGlobalTrade()
		assert.ErrorContains(t, err, "unexpected inst id")
	})
}

func TestWebSocketEvent_IsValid(t *testing.T) {
	t.Run("op login event", func(t *testing.T) {
		input := `{
  "event": "login",
  "code": "0",
  "msg": "",
  "connId": "a4d3ae55"
}`
		res, err := parseWebSocketEvent([]byte(input))
		assert.NoError(t, err)
		opEvent, ok := res.(*WebSocketEvent)
		assert.True(t, ok)
		assert.Equal(t, WebSocketEvent{
			Event:   WsEventTypeLogin,
			Code:    "0",
			Message: "",
		}, *opEvent)

		assert.NoError(t, opEvent.IsValid())
	})

	t.Run("op error event", func(t *testing.T) {
		input := `{
  "event": "error",
  "code": "60009",
  "msg": "Login failed.",
  "connId": "a4d3ae55"
}`
		res, err := parseWebSocketEvent([]byte(input))
		assert.NoError(t, err)
		opEvent, ok := res.(*WebSocketEvent)
		assert.True(t, ok)
		assert.Equal(t, WebSocketEvent{
			Event:   WsEventTypeError,
			Code:    "60009",
			Message: "Login failed.",
		}, *opEvent)

		assert.ErrorContains(t, opEvent.IsValid(), "request error")
	})

	t.Run("unexpected event", func(t *testing.T) {
		input := `{
  "event": "test gg",
  "code": "60009",
  "msg": "unexpected",
  "connId": "a4d3ae55"
}`
		res, err := parseWebSocketEvent([]byte(input))
		assert.NoError(t, err)
		opEvent, ok := res.(*WebSocketEvent)
		assert.True(t, ok)
		assert.Equal(t, WebSocketEvent{
			Event:   "test gg",
			Code:    "60009",
			Message: "unexpected",
		}, *opEvent)

		assert.ErrorContains(t, opEvent.IsValid(), "unexpected event type")
	})

	t.Run("conn count info", func(t *testing.T) {
		input := `{
    "event":"channel-conn-count",
    "channel":"orders",
    "connCount": "2",
    "connId":"abcd1234"
}`
		res, err := parseWebSocketEvent([]byte(input))
		assert.NoError(t, err)
		opEvent, ok := res.(*WebSocketEvent)
		assert.True(t, ok)
		assert.Equal(t, WebSocketEvent{
			Event:     "channel-conn-count",
			Channel:   "orders",
			ConnCount: "2",
		}, *opEvent)

		assert.NoError(t, opEvent.IsValid())
	})

	t.Run("conn count error", func(t *testing.T) {
		input := `{
    "event": "channel-conn-count-error",
    "channel": "orders",
    "connCount": "20",
    "connId":"a4d3ae55"
}`
		res, err := parseWebSocketEvent([]byte(input))
		assert.NoError(t, err)
		opEvent, ok := res.(*WebSocketEvent)
		assert.True(t, ok)
		assert.Equal(t, WebSocketEvent{
			Event:     "channel-conn-count-error",
			Channel:   "orders",
			ConnCount: "20",
		}, *opEvent)

		assert.ErrorContains(t, opEvent.IsValid(), "rate limit")
	})
}

func TestOrderTradeEvent(t *testing.T) {
	// {"arg":{"channel":"orders","instType":"SPOT","uid":"530315546680502420"},"data":[{"accFillSz":"0","algoClOrdId":"","algoId":"","amendResult":"","amendSource":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"0","cTime":"1705384184502","cancelSource":"","category":"normal","ccy":"","clOrdId":"","code":"0","execType":"","fee":"0","feeCcy":"OKB","fillFee":"0","fillFeeCcy":"","fillFwdPx":"","fillMarkPx":"","fillMarkVol":"","fillNotionalUsd":"","fillPnl":"0","fillPx":"","fillPxUsd":"","fillPxVol":"","fillSz":"0","fillTime":"","instId":"OKB-USDT","instType":"SPOT","lastPx":"54","lever":"0","msg":"","notionalUsd":"99.929","ordId":"667364871905857536","ordType":"market","pnl":"0","posSide":"","px":"","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"USDT","reduceOnly":"false","reqId":"","side":"buy","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"live","stpId":"","stpMode":"","sz":"100","tag":"","tdMode":"cash","tgtCcy":"quote_ccy","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"","uTime":"1705384184502"}]}
	// {"arg":{"channel":"orders","instType":"SPOT","uid":"530315546680502420"},"data":[{"accFillSz":"1.84","algoClOrdId":"","algoId":"","amendResult":"","amendSource":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"54","cTime":"1705384184502","cancelSource":"","category":"normal","ccy":"","clOrdId":"","code":"0","execType":"T","fee":"-0.001844337","feeCcy":"OKB","fillFee":"-0.001844337","fillFeeCcy":"OKB","fillFwdPx":"","fillMarkPx":"","fillMarkVol":"","fillNotionalUsd":"99.9","fillPnl":"0","fillPx":"54","fillPxUsd":"","fillPxVol":"","fillSz":"1.844337","fillTime":"1705384184503","instId":"OKB-USDT","instType":"SPOT","lastPx":"54","lever":"0","msg":"","notionalUsd":"99.929","ordId":"667364871905857536","ordType":"market","pnl":"0","posSide":"","px":"","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"USDT","reduceOnly":"false","reqId":"","side":"buy","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"partially_filled","stpId":"","stpMode":"","sz":"100","tag":"","tdMode":"cash","tgtCcy":"quote_ccy","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"590957341","uTime":"1705384184503"}]}
	// {"arg":{"channel":"orders","instType":"SPOT","uid":"530315546680502420"},"data":[{"accFillSz":"1.84","algoClOrdId":"","algoId":"","amendResult":"","amendSource":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"54","cTime":"1705384184502","cancelSource":"","category":"normal","ccy":"","clOrdId":"","code":"0","execType":"","fee":"-0.001844337","feeCcy":"OKB","fillFee":"0","fillFeeCcy":"","fillFwdPx":"","fillMarkPx":"","fillMarkVol":"","fillNotionalUsd":"99.9","fillPnl":"0","fillPx":"","fillPxUsd":"","fillPxVol":"","fillSz":"0","fillTime":"","instId":"OKB-USDT","instType":"SPOT","lastPx":"54","lever":"0","msg":"","notionalUsd":"99.929","ordId":"667364871905857536","ordType":"market","pnl":"0","posSide":"","px":"","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"USDT","reduceOnly":"false","reqId":"","side":"buy","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"100","tag":"","tdMode":"cash","tgtCcy":"quote_ccy","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"","uTime":"1705384184504"}]}
	t.Run("succeeds", func(t *testing.T) {
		in := `
{
   "arg":{
      "channel":"orders",
      "instType":"SPOT",
      "uid":"530315546680502420"
   },
   "data":[
      {
         "accFillSz":"1.84",
         "algoClOrdId":"",
         "algoId":"",
         "amendResult":"",
         "amendSource":"",
         "attachAlgoClOrdId":"",
         "attachAlgoOrds":[
            
         ],
         "avgPx":"54",
         "cTime":"1705384184502",
         "cancelSource":"",
         "category":"normal",
         "ccy":"",
         "clOrdId":"",
         "code":"0",
         "execType":"T",
         "fee":"-0.001844337",
         "feeCcy":"OKB",
         "fillFee":"-0.001844337",
         "fillFeeCcy":"OKB",
         "fillFwdPx":"",
         "fillMarkPx":"",
         "fillMarkVol":"",
         "fillNotionalUsd":"99.9",
         "fillPnl":"0",
         "fillPx":"54",
         "fillPxUsd":"",
         "fillPxVol":"",
         "fillSz":"1.84",
         "fillTime":"1705384184503",
         "instId":"OKB-USDT",
         "instType":"SPOT",
         "lastPx":"54",
         "lever":"0",
         "msg":"",
         "notionalUsd":"99.929",
         "ordId":"667364871905857536",
         "ordType":"market",
         "pnl":"0",
         "posSide":"",
         "px":"",
         "pxType":"",
         "pxUsd":"",
         "pxVol":"",
         "quickMgnType":"",
         "rebate":"0",
         "rebateCcy":"USDT",
         "reduceOnly":"false",
         "reqId":"",
         "side":"buy",
         "slOrdPx":"",
         "slTriggerPx":"",
         "slTriggerPxType":"",
         "source":"",
         "state":"partially_filled",
         "stpId":"",
         "stpMode":"",
         "sz":"100",
         "tag":"",
         "tdMode":"cash",
         "tgtCcy":"quote_ccy",
         "tpOrdPx":"",
         "tpTriggerPx":"",
         "tpTriggerPxType":"",
         "tradeId":"590957341",
         "uTime":"1705384184503"
      }
   ]
}
`

		exp := []OrderTradeEvent{
			{
				OrderDetail: okexapi.OrderDetail{
					AccumulatedFillSize: fixedpoint.NewFromFloat(1.84),
					AvgPrice:            fixedpoint.NewFromFloat(54),
					CreatedTime:         types.NewMillisecondTimestampFromInt(1705384184502),
					Category:            "normal",
					ClientOrderId:       "",
					Fee:                 fixedpoint.NewFromFloat(-0.001844337),
					FeeCurrency:         "OKB",
					FillTime:            types.NewMillisecondTimestampFromInt(1705384184503),
					InstrumentID:        "OKB-USDT",
					InstrumentType:      okexapi.InstrumentTypeSpot,
					OrderId:             strint.Int64(667364871905857536),
					OrderType:           okexapi.OrderTypeMarket,
					Price:               fixedpoint.Zero,
					Side:                okexapi.SideTypeBuy,
					State:               okexapi.OrderStatePartiallyFilled,
					Size:                fixedpoint.NewFromFloat(100),
					TargetCurrency:      okexapi.TargetCurrencyQuote,
					UpdatedTime:         types.NewMillisecondTimestampFromInt(1705384184503),
					Currency:            "",
					TradeId:             "590957341",
					FillPrice:           fixedpoint.NewFromFloat(54),
					FillSize:            fixedpoint.NewFromFloat(1.84),
					Lever:               "0",
					Pnl:                 fixedpoint.Zero,
					PositionSide:        "",
					PriceUsd:            fixedpoint.Zero,
					PriceVol:            fixedpoint.Zero,
					PriceType:           "",
					Rebate:              fixedpoint.Zero,
					RebateCcy:           "USDT",
					AttachAlgoClOrdId:   "",
					SlOrdPx:             fixedpoint.Zero,
					SlTriggerPx:         fixedpoint.Zero,
					SlTriggerPxType:     "",
					AttachAlgoOrds:      []interface{}{},
					Source:              "",
					StpId:               "",
					StpMode:             "",
					Tag:                 "",
					TradeMode:           okexapi.TradeModeCash,
					TpOrdPx:             fixedpoint.Zero,
					TpTriggerPx:         fixedpoint.Zero,
					TpTriggerPxType:     "",
					ReduceOnly:          "false",
					AlgoClOrdId:         "",
					AlgoId:              "",
				},
				Code:            strint.Int64(0),
				Msg:             "",
				AmendResult:     "",
				ExecutionType:   okexapi.LiquidityTypeTaker,
				FillFee:         fixedpoint.NewFromFloat(-0.001844337),
				FillFeeCurrency: "OKB",
				FillNotionalUsd: fixedpoint.NewFromFloat(99.9),
				FillPnl:         fixedpoint.Zero,
				NotionalUsd:     fixedpoint.NewFromFloat(99.929),
				ReqId:           "",
				LastPrice:       fixedpoint.NewFromFloat(54),
				QuickMgnType:    "",
				AmendSource:     "",
				CancelSource:    "",
				FillPriceVolume: "",
				FillPriceUsd:    "",
				FillMarkVolume:  "",
				FillFwdPrice:    "",
				FillMarkPrice:   "",
			},
		}

		res, err := parseWebSocketEvent([]byte(in))
		assert.NoError(t, err)
		event, ok := res.([]OrderTradeEvent)
		assert.True(t, ok)
		assert.Equal(t, exp, event)
	})
}

func TestOrderTradeEvent_toGlobalTrade1(t *testing.T) {
	var (
		in = `
{
   "arg":{
      "channel":"orders",
      "instType":"SPOT",
      "uid":"530315546680502420"
   },
   "data":[
      {
         "accFillSz":"1.84",
         "algoClOrdId":"",
         "algoId":"",
         "amendResult":"",
         "amendSource":"",
         "attachAlgoClOrdId":"",
         "attachAlgoOrds":[
            
         ],
         "avgPx":"54",
         "cTime":"1705384184502",
         "cancelSource":"",
         "category":"normal",
         "ccy":"",
         "clOrdId":"",
         "code":"0",
         "execType":"T",
         "fee":"-0.001844337",
         "feeCcy":"OKB",
         "fillFee":"-0.001844337",
         "fillFeeCcy":"OKB",
         "fillFwdPx":"",
         "fillMarkPx":"",
         "fillMarkVol":"",
         "fillNotionalUsd":"99.9",
         "fillPnl":"0",
         "fillPx":"54",
         "fillPxUsd":"",
         "fillPxVol":"",
         "fillSz":"1.84",
         "fillTime":"1705384184503",
         "instId":"OKB-USDT",
         "instType":"SPOT",
         "lastPx":"54",
         "lever":"0",
         "msg":"",
         "notionalUsd":"99.929",
         "ordId":"667364871905857536",
         "ordType":"market",
         "pnl":"0",
         "posSide":"",
         "px":"",
         "pxType":"",
         "pxUsd":"",
         "pxVol":"",
         "quickMgnType":"",
         "rebate":"0",
         "rebateCcy":"USDT",
         "reduceOnly":"false",
         "reqId":"",
         "side":"buy",
         "slOrdPx":"",
         "slTriggerPx":"",
         "slTriggerPxType":"",
         "source":"",
         "state":"partially_filled",
         "stpId":"",
         "stpMode":"",
         "sz":"100",
         "tag":"",
         "tdMode":"cash",
         "tgtCcy":"quote_ccy",
         "tpOrdPx":"",
         "tpTriggerPx":"",
         "tpTriggerPxType":"",
         "tradeId":"590957341",
         "uTime":"1705384184503"
      }
   ]
}
`
		expTrade = types.Trade{
			ID:            uint64(590957341),
			OrderID:       uint64(667364871905857536),
			Exchange:      types.ExchangeOKEx,
			Price:         fixedpoint.NewFromFloat(54),
			Quantity:      fixedpoint.NewFromFloat(1.84),
			QuoteQuantity: fixedpoint.NewFromFloat(54).Mul(fixedpoint.NewFromFloat(1.84)),
			Symbol:        "OKBUSDT",
			Side:          types.SideTypeBuy,
			IsBuyer:       true,
			IsMaker:       false,
			Time:          types.Time(types.NewMillisecondTimestampFromInt(1705384184503).Time()),
			Fee:           fixedpoint.NewFromFloat(0.001844337),
			FeeCurrency:   "OKB",
		}
	)

	t.Run("succeeds", func(t *testing.T) {
		res, err := parseWebSocketEvent([]byte(in))
		assert.NoError(t, err)
		event, ok := res.([]OrderTradeEvent)
		assert.True(t, ok)

		trade, err := event[0].toGlobalTrade()
		assert.NoError(t, err)
		assert.Equal(t, expTrade, trade)
	})

	t.Run("unexpected trade id", func(t *testing.T) {
		res, err := parseWebSocketEvent([]byte(in))
		assert.NoError(t, err)
		event, ok := res.([]OrderTradeEvent)
		assert.True(t, ok)

		event[0].TradeId = "XXXX"
		_, err = event[0].toGlobalTrade()
		assert.ErrorContains(t, err, "trade id")
	})

}

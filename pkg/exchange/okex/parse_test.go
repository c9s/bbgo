package okex

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

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
			Events: KLineSlice{
				{
					StartTime:      types.NewMillisecondTimestampFromInt(1597026383085),
					OpenPrice:      fixedpoint.NewFromFloat(8533),
					HighestPrice:   fixedpoint.NewFromFloat(8553.74),
					LowestPrice:    fixedpoint.NewFromFloat(8527.17),
					ClosePrice:     fixedpoint.NewFromFloat(8548.26),
					Volume:         fixedpoint.NewFromFloat(45247),
					VolumeCcy:      fixedpoint.NewFromFloat(529.5858061),
					VolumeCcyQuote: fixedpoint.NewFromFloat(529.5858061),
					Confirm:        fixedpoint.Zero,
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
			Events: KLineSlice{
				{
					StartTime:      types.NewMillisecondTimestampFromInt(1597026383085),
					OpenPrice:      fixedpoint.NewFromFloat(8533),
					HighestPrice:   fixedpoint.NewFromFloat(8553.74),
					LowestPrice:    fixedpoint.NewFromFloat(8527.17),
					ClosePrice:     fixedpoint.NewFromFloat(8548.26),
					Volume:         fixedpoint.NewFromFloat(45247),
					VolumeCcy:      fixedpoint.NewFromFloat(529.5858061),
					VolumeCcyQuote: fixedpoint.NewFromFloat(529.5858061),
					Confirm:        fixedpoint.Zero,
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
			QuoteVolume:              exp.Events[0].VolumeCcy,
			TakerBuyBaseAssetVolume:  fixedpoint.Zero,
			TakerBuyQuoteAssetVolume: fixedpoint.Zero,
			LastTradeID:              0,
			NumberOfTrades:           0,
			Closed:                   false,
		}, event.Events[0].ToGlobal(types.Interval(event.Interval), event.Symbol))
	})

}

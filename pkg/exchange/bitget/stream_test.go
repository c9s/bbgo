package bitget

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func getTestClientOrSkip(t *testing.T) *Stream {
	if b, _ := strconv.ParseBool(os.Getenv("CI")); b {
		t.Skip("skip test for CI")
	}

	return NewStream(os.Getenv("BITGET_API_KEY"),
		os.Getenv("BITGET_API_SECRET"),
		os.Getenv("BITGET_API_PASSPHRASE"))
}

func TestStream(t *testing.T) {
	t.Skip()
	s := getTestClientOrSkip(t)

	symbols := []string{
		"BTCUSDT",
		"ETHUSDT",
		"DOTUSDT",
		"ADAUSDT",
		"AAVEUSDT",
		"APTUSDT",
		"ATOMUSDT",
		"AXSUSDT",
		"BNBUSDT",
		"SOLUSDT",
		"DOGEUSDT",
	}

	t.Run("book test", func(t *testing.T) {
		s.Subscribe(types.BookChannel, "BTCUSDT", types.SubscribeOptions{
			Depth: types.DepthLevel5,
		})
		s.SetPublicOnly()
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnBookSnapshot(func(book types.SliceOrderBook) {
			t.Log("got snapshot", len(book.Bids), len(book.Asks), book.Symbol, book.Time, book)
		})
		s.OnBookUpdate(func(book types.SliceOrderBook) {
			t.Log("got update", len(book.Bids), len(book.Asks), book.Symbol, book.Time, book)
		})
		c := make(chan struct{})
		<-c
	})

	t.Run("book test on unsubscribe and reconnect", func(t *testing.T) {
		for _, symbol := range symbols {
			s.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{
				Depth: types.DepthLevel200,
			})
		}

		s.SetPublicOnly()
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnBookSnapshot(func(book types.SliceOrderBook) {
			t.Log("got snapshot", book)
		})
		s.OnBookUpdate(func(book types.SliceOrderBook) {
			t.Log("got update", book)
		})

		<-time.After(2 * time.Second)

		s.Unsubscribe()
		for _, symbol := range symbols {
			s.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{
				Depth: types.DepthLevel200,
			})
		}

		<-time.After(2 * time.Second)

		s.Reconnect()

		c := make(chan struct{})
		<-c
	})

	t.Run("trade test", func(t *testing.T) {
		s.Subscribe(types.MarketTradeChannel, "BTCUSDT", types.SubscribeOptions{})
		s.SetPublicOnly()
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnMarketTrade(func(trade types.Trade) {
			t.Log("got update", trade)
		})
		c := make(chan struct{})
		<-c
	})

	t.Run("kline test", func(t *testing.T) {
		s.Subscribe(types.KLineChannel, "BTCUSDT", types.SubscribeOptions{Interval: types.Interval1w})
		s.SetPublicOnly()
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnKLine(func(kline types.KLine) {
			t.Log("got update", kline)
		})
		s.OnKLineClosed(func(kline types.KLine) {
			t.Log("got closed update", kline)
		})
		c := make(chan struct{})
		<-c
	})

	t.Run("private test", func(t *testing.T) {
		s.SetPrivateChannelSymbols([]string{"BTCUSDT"})
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnBalanceSnapshot(func(balances types.BalanceMap) {
			t.Log("get balances", balances)
		})
		s.OnBalanceUpdate(func(balances types.BalanceMap) {
			t.Log("get update", balances)
		})
		s.OnOrderUpdate(func(order types.Order) {
			t.Log("order update", order)
		})
		s.OnTradeUpdate(func(trade types.Trade) {
			t.Log("trade update", trade)
		})

		c := make(chan struct{})
		<-c
	})

}

func TestStream_parseWebSocketEvent(t *testing.T) {
	t.Run("op subscribe event", func(t *testing.T) {
		input := `{
		   "event":"subscribe",
		   "arg":{
			  "instType":"sp",
			  "channel":"books5",
			  "instId":"BTCUSDT"
		   }
		}`
		res, err := parseWebSocketEvent([]byte(input))
		assert.NoError(t, err)
		opEvent, ok := res.(*WsEvent)
		assert.True(t, ok)
		assert.Equal(t, WsEvent{
			Event: WsEventSubscribe,
			Arg: WsArg{
				InstType: instSp,
				Channel:  ChannelOrderBook5,
				InstId:   "BTCUSDT",
			},
		}, *opEvent)

		assert.NoError(t, opEvent.IsValid())
	})

	t.Run("op unsubscribe event", func(t *testing.T) {
		input := `{
		   "event":"unsubscribe",
		   "arg":{
			  "instType":"sp",
			  "channel":"books5",
			  "instId":"BTCUSDT"
		   }
		}`
		res, err := parseWebSocketEvent([]byte(input))
		assert.NoError(t, err)
		opEvent, ok := res.(*WsEvent)
		assert.True(t, ok)
		assert.Equal(t, WsEvent{
			Event: WsEventUnsubscribe,
			Arg: WsArg{
				InstType: instSp,
				Channel:  ChannelOrderBook5,
				InstId:   "BTCUSDT",
			},
		}, *opEvent)
	})

	t.Run("op error event", func(t *testing.T) {
		input := `{
		   "event":"error",
		   "arg":{
			  "instType":"sp",
			  "channel":"books5",
			  "instId":"BTCUSDT-"
		   },
		   "code":30001,
		   "msg":"instType:sp,channel:books5,instId:BTCUSDT- doesn't exist",
		   "op":"subscribe"
		}`
		res, err := parseWebSocketEvent([]byte(input))
		assert.NoError(t, err)
		opEvent, ok := res.(*WsEvent)
		assert.True(t, ok)
		assert.Equal(t, WsEvent{
			Event: WsEventError,
			Code:  30001,
			Msg:   "instType:sp,channel:books5,instId:BTCUSDT- doesn't exist",
			Op:    "subscribe",
			Arg: WsArg{
				InstType: instSp,
				Channel:  ChannelOrderBook5,
				InstId:   "BTCUSDT-",
			},
		}, *opEvent)
	})

	t.Run("Orderbook event", func(t *testing.T) {
		input := `{
		   "action":"%s",
		   "arg":{
			  "instType":"sp",
			  "channel":"books5",
			  "instId":"BTCUSDT"
		   },
		   "data":[
			  {
				 "asks":[
					[
					   "28350.78",
					   "0.2082"
					],
					[
					   "28350.80",
					   "0.2081"
					]
				 ],
				 "bids":[
					[
					   "28350.70",
					   "0.5585"
					],
					[
					   "28350.67",
					   "6.8175"
					]
				 ],
				 "checksum":0,
				 "ts":"1697593934630"
			  }
		   ],
		   "ts":1697593934630
		}`

		eventFn := func(in string, actionType ActionType) {
			res, err := parseWebSocketEvent([]byte(in))
			assert.NoError(t, err)
			book, ok := res.(*BookEvent)
			assert.True(t, ok)
			assert.Equal(t, BookEvent{
				Events: []struct {
					Asks types.PriceVolumeSlice `json:"asks"`
					// Order book on buy side, descending order
					Bids     types.PriceVolumeSlice     `json:"bids"`
					Ts       types.MillisecondTimestamp `json:"ts"`
					Checksum int                        `json:"checksum"`
				}{
					{
						Asks: []types.PriceVolume{
							{
								Price:  fixedpoint.NewFromFloat(28350.78),
								Volume: fixedpoint.NewFromFloat(0.2082),
							},
							{
								Price:  fixedpoint.NewFromFloat(28350.80),
								Volume: fixedpoint.NewFromFloat(0.2081),
							},
						},
						Bids: []types.PriceVolume{
							{
								Price:  fixedpoint.NewFromFloat(28350.70),
								Volume: fixedpoint.NewFromFloat(0.5585),
							},
							{
								Price:  fixedpoint.NewFromFloat(28350.67),
								Volume: fixedpoint.NewFromFloat(6.8175),
							},
						},
						Ts:       types.NewMillisecondTimestampFromInt(1697593934630),
						Checksum: 0,
					},
				},
				actionType: actionType,
				instId:     "BTCUSDT",
			}, *book)
		}

		t.Run("snapshot type", func(t *testing.T) {
			snapshotInput := fmt.Sprintf(input, ActionTypeSnapshot)
			eventFn(snapshotInput, ActionTypeSnapshot)
		})

		t.Run("update type", func(t *testing.T) {
			snapshotInput := fmt.Sprintf(input, ActionTypeUpdate)
			eventFn(snapshotInput, ActionTypeUpdate)
		})
	})
}

func Test_parseWebSocketEvent_MarketTrade(t *testing.T) {
	t.Run("MarketTrade event", func(t *testing.T) {
		input := `{
		   "action":"%s",
		   "arg":{
			  "instType":"sp",
			  "channel":"trade",
			  "instId":"BTCUSDT"
		   },
		   "data":[
			  [
				 "1697697791663",
				 "28303.43",
				 "0.0452",
				 "sell"
			  ],
			  [
				 "1697697794663",
				 "28345.67",
				 "0.1234",
				 "sell"
			  ]
		   ],
		   "ts":1697697791670
		}`

		eventFn := func(in string, actionType ActionType) {
			res, err := parseWebSocketEvent([]byte(in))
			assert.NoError(t, err)
			book, ok := res.(*MarketTradeEvent)
			assert.True(t, ok)
			assert.Equal(t, MarketTradeEvent{
				Events: MarketTradeSlice{
					{
						Ts:    types.NewMillisecondTimestampFromInt(1697697791663),
						Price: fixedpoint.NewFromFloat(28303.43),
						Size:  fixedpoint.NewFromFloat(0.0452),
						Side:  "sell",
					},

					{
						Ts:    types.NewMillisecondTimestampFromInt(1697697794663),
						Price: fixedpoint.NewFromFloat(28345.67),
						Size:  fixedpoint.NewFromFloat(0.1234),
						Side:  "sell",
					},
				},
				actionType: actionType,
				instId:     "BTCUSDT",
			}, *book)
		}

		t.Run("snapshot type", func(t *testing.T) {
			snapshotInput := fmt.Sprintf(input, ActionTypeSnapshot)
			eventFn(snapshotInput, ActionTypeSnapshot)
		})

		t.Run("update type", func(t *testing.T) {
			snapshotInput := fmt.Sprintf(input, ActionTypeUpdate)
			eventFn(snapshotInput, ActionTypeUpdate)
		})
	})

	t.Run("Unexpected length of market trade", func(t *testing.T) {
		input := `{
		   "action":"%s",
		   "arg":{
			  "instType":"sp",
			  "channel":"trade",
			  "instId":"BTCUSDT"
		   },
		   "data":[
			  [
				 "1697697791663",
				 "28303.43",
				 "28303.43",
				 "0.0452",
				 "sell"
			  ]
		   ],
		   "ts":1697697791670
		}`
		_, err := parseWebSocketEvent([]byte(input))
		assert.ErrorContains(t, err, "unexpected trades length")
	})

	t.Run("Unexpected timestamp", func(t *testing.T) {
		input := `{
		   "action":"%s",
		   "arg":{
			  "instType":"sp",
			  "channel":"trade",
			  "instId":"BTCUSDT"
		   },
		   "data":[
			  [
				 "TIMESTAMP",
				 "28303.43",
				 "0.0452",
				 "sell"
			  ]
		   ],
		   "ts":1697697791670
		}`
		_, err := parseWebSocketEvent([]byte(input))
		assert.ErrorContains(t, err, "timestamp")
	})

	t.Run("Unexpected price", func(t *testing.T) {
		input := `{
		   "action":"%s",
		   "arg":{
			  "instType":"sp",
			  "channel":"trade",
			  "instId":"BTCUSDT"
		   },
		   "data":[
			  [
				 "1697697791663",
				 "1p",
				 "0.0452",
				 "sell"
			  ]
		   ],
		   "ts":1697697791670
		}`
		_, err := parseWebSocketEvent([]byte(input))
		assert.ErrorContains(t, err, "price")
	})

	t.Run("Unexpected size", func(t *testing.T) {
		input := `{
		   "action":"%s",
		   "arg":{
			  "instType":"sp",
			  "channel":"trade",
			  "instId":"BTCUSDT"
		   },
		   "data":[
			  [
				 "1697697791663",
				 "28303.43",
				 "2v",
				 "sell"
			  ]
		   ],
		   "ts":1697697791670
		}`
		_, err := parseWebSocketEvent([]byte(input))
		assert.ErrorContains(t, err, "size")
	})

	t.Run("Unexpected side", func(t *testing.T) {
		input := `{
		   "action":"%s",
		   "arg":{
			  "instType":"sp",
			  "channel":"trade",
			  "instId":"BTCUSDT"
		   },
		   "data":[
			  [
				 "1697697791663",
				 "28303.43",
				 "0.0452",
				 12345
			  ]
		   ],
		   "ts":1697697791670
		}`
		_, err := parseWebSocketEvent([]byte(input))
		assert.ErrorContains(t, err, "side")
	})
}

func Test_parseWebSocketEvent_KLine(t *testing.T) {
	t.Run("KLine event", func(t *testing.T) {
		input := `{
		   "action":"%s",
		   "arg":{
			  "instType":"sp",
			  "channel":"candle5m",
			  "instId":"BTCUSDT"
		   },
		   "data":[
				["1698744600000","34361.49","34458.98","34355.53","34416.41","99.6631"]
		   ],
		   "ts":1697697791670
		}`

		eventFn := func(in string, actionType ActionType) {
			res, err := parseWebSocketEvent([]byte(in))
			assert.NoError(t, err)
			kline, ok := res.(*KLineEvent)
			assert.True(t, ok)
			assert.Equal(t, KLineEvent{
				channel: "candle5m",
				Events: KLineSlice{
					{
						StartTime:    types.NewMillisecondTimestampFromInt(1698744600000),
						OpenPrice:    fixedpoint.NewFromFloat(34361.49),
						HighestPrice: fixedpoint.NewFromFloat(34458.98),
						LowestPrice:  fixedpoint.NewFromFloat(34355.53),
						ClosePrice:   fixedpoint.NewFromFloat(34416.41),
						Volume:       fixedpoint.NewFromFloat(99.6631),
					},
				},
				actionType: actionType,
				instId:     "BTCUSDT",
			}, *kline)
		}

		t.Run("snapshot type", func(t *testing.T) {
			snapshotInput := fmt.Sprintf(input, ActionTypeSnapshot)
			eventFn(snapshotInput, ActionTypeSnapshot)
		})

		t.Run("update type", func(t *testing.T) {
			snapshotInput := fmt.Sprintf(input, ActionTypeUpdate)
			eventFn(snapshotInput, ActionTypeUpdate)
		})
	})

	t.Run("Unexpected length of kline", func(t *testing.T) {
		input := `{
		   "action":"%s",
		   "arg":{
			  "instType":"sp",
			  "channel":"candle5m",
			  "instId":"BTCUSDT"
		   },
		   "data":[
			  ["1698744600000","34361.45","34458.98","34355.53","34416.41","99.6631", "123456"]
		   ],
		   "ts":1697697791670
		}`
		_, err := parseWebSocketEvent([]byte(input))
		assert.ErrorContains(t, err, "unexpected kline length")
	})

	t.Run("Unexpected timestamp", func(t *testing.T) {
		input := `{
		   "action":"%s",
		   "arg":{
			  "instType":"sp",
			  "channel":"candle5m",
			  "instId":"BTCUSDT"
		   },
		   "data":[
			  ["timestamp","34361.49","34458.98","34355.53","34416.41","99.6631"]
		   ],
		   "ts":1697697791670
		}`
		_, err := parseWebSocketEvent([]byte(input))
		assert.ErrorContains(t, err, "timestamp")
	})

	t.Run("Unexpected open price", func(t *testing.T) {
		input := `{
		   "action":"%s",
		   "arg":{
			  "instType":"sp",
			  "channel":"candle5m",
			  "instId":"BTCUSDT"
		   },
		   "data":[
				["1698744600000","1p","34458.98","34355.53","34416.41","99.6631"]
		   ],
		   "ts":1697697791670
		}`
		_, err := parseWebSocketEvent([]byte(input))
		assert.ErrorContains(t, err, "open price")
	})

	t.Run("Unexpected highest price", func(t *testing.T) {
		input := `{
		   "action":"%s",
		   "arg":{
			  "instType":"sp",
			  "channel":"candle5m",
			  "instId":"BTCUSDT"
		   },
		   "data":[
			  ["1698744600000","34361.45","3p","34355.53","34416.41","99.6631"]
		   ],
		   "ts":1697697791670
		}`
		_, err := parseWebSocketEvent([]byte(input))
		assert.ErrorContains(t, err, "highest price")
	})

	t.Run("Unexpected lowest price", func(t *testing.T) {
		input := `{
		   "action":"%s",
		   "arg":{
			  "instType":"sp",
			  "channel":"candle5m",
			  "instId":"BTCUSDT"
		   },
		   "data":[
			  ["1698744600000","34361.45","34458.98","1p","34416.41","99.6631"]
		   ],
		   "ts":1697697791670
		}`
		_, err := parseWebSocketEvent([]byte(input))
		assert.ErrorContains(t, err, "lowest price")
	})

	t.Run("Unexpected close price", func(t *testing.T) {
		input := `{
		   "action":"%s",
		   "arg":{
			  "instType":"sp",
			  "channel":"candle5m",
			  "instId":"BTCUSDT"
		   },
		   "data":[
			  ["1698744600000","34361.45","34458.98","34355.53","1c","99.6631"]
		   ],
		   "ts":1697697791670
		}`
		_, err := parseWebSocketEvent([]byte(input))
		assert.ErrorContains(t, err, "close price")
	})

	t.Run("Unexpected volume", func(t *testing.T) {
		input := `{
		   "action":"%s",
		   "arg":{
			  "instType":"sp",
			  "channel":"candle5m",
			  "instId":"BTCUSDT"
		   },
		   "data":[
			  ["1698744600000","34361.45","34458.98","34355.53","34416.41", "1v"]
		   ],
		   "ts":1697697791670
		}`
		_, err := parseWebSocketEvent([]byte(input))
		assert.ErrorContains(t, err, "volume")
	})
}

func Test_convertSubscription(t *testing.T) {
	t.Run("BookChannel.ChannelOrderBook5", func(t *testing.T) {
		res, err := convertSubscription(types.Subscription{
			Symbol:  "BTCUSDT",
			Channel: types.BookChannel,
			Options: types.SubscribeOptions{
				Depth: types.DepthLevel5,
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, WsArg{
			InstType: instSp,
			Channel:  ChannelOrderBook5,
			InstId:   "BTCUSDT",
		}, res)
	})
	t.Run("BookChannel.DepthLevel15", func(t *testing.T) {
		res, err := convertSubscription(types.Subscription{
			Symbol:  "BTCUSDT",
			Channel: types.BookChannel,
			Options: types.SubscribeOptions{
				Depth: types.DepthLevel15,
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, WsArg{
			InstType: instSp,
			Channel:  ChannelOrderBook15,
			InstId:   "BTCUSDT",
		}, res)
	})
	t.Run("BookChannel.DepthLevel200", func(t *testing.T) {
		res, err := convertSubscription(types.Subscription{
			Symbol:  "BTCUSDT",
			Channel: types.BookChannel,
			Options: types.SubscribeOptions{
				Depth: types.DepthLevel200,
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, WsArg{
			InstType: instSp,
			Channel:  ChannelOrderBook,
			InstId:   "BTCUSDT",
		}, res)
	})
	t.Run("TradeChannel", func(t *testing.T) {
		res, err := convertSubscription(types.Subscription{
			Symbol:  "BTCUSDT",
			Channel: types.MarketTradeChannel,
			Options: types.SubscribeOptions{},
		})
		assert.NoError(t, err)
		assert.Equal(t, WsArg{
			InstType: instSp,
			Channel:  ChannelTrade,
			InstId:   "BTCUSDT",
		}, res)
	})
	t.Run("CandleChannel", func(t *testing.T) {
		for gInterval, localInterval := range toLocalInterval {
			res, err := convertSubscription(types.Subscription{
				Symbol:  "BTCUSDT",
				Channel: types.KLineChannel,
				Options: types.SubscribeOptions{
					Interval: gInterval,
				},
			})
			assert.NoError(t, err)
			assert.Equal(t, WsArg{
				InstType: instSp,
				Channel:  ChannelType(localInterval),
				InstId:   "BTCUSDT",
			}, res)
		}
	})
}

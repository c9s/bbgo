package bybit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
)

func getTestClientOrSkip(t *testing.T) *Stream {
	if b, _ := strconv.ParseBool(os.Getenv("CI")); b {
		t.Skip("skip test for CI")
	}

	key, secret, ok := testutil.IntegrationTestConfigured(t, "BYBIT")
	if !ok {
		t.Skip("BYBIT_* env vars are not configured")
		return nil
	}

	exchange, err := New(key, secret)
	assert.NoError(t, err)
	return NewStream(key, secret, exchange)
}

func TestStream(t *testing.T) {
	//t.Skip()
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

	t.Run("Auth test", func(t *testing.T) {
		s.OnBalanceSnapshot(func(balances types.BalanceMap) {
			t.Log("got balance snapshot", balances)
		})
		s.Connect(context.Background())
		c := make(chan struct{})
		<-c
	})

	t.Run("book test", func(t *testing.T) {
		s.Subscribe(types.BookChannel, "BTCUSDT", types.SubscribeOptions{
			Depth: types.DepthLevel50,
		})
		s.SetPublicOnly()
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		//s.OnBookSnapshot(func(book types.SliceOrderBook) {
		//	t.Log("got snapshot", book)
		//})
		//s.OnBookUpdate(func(book types.SliceOrderBook) {
		//	t.Log("got update", book)
		//})
		c := make(chan struct{})
		<-c
	})

	t.Run("book test on unsubscribe and reconnect", func(t *testing.T) {
		for _, symbol := range symbols {
			s.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{
				Depth: types.DepthLevel50,
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
				Depth: types.DepthLevel50,
			})
		}

		<-time.After(2 * time.Second)

		s.Reconnect()

		c := make(chan struct{})
		<-c
	})

	t.Run("market trade test", func(t *testing.T) {
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

	t.Run("wallet test", func(t *testing.T) {
		s.OnAuth(func() {
			t.Log("authenticated")
		})
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnBalanceUpdate(func(balances types.BalanceMap) {
			t.Log("got update", balances)
		})
		c := make(chan struct{})
		<-c
	})

	t.Run("order test", func(t *testing.T) {
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnOrderUpdate(func(order types.Order) {
			t.Log("got update", order)
		})
		c := make(chan struct{})
		<-c
	})

	t.Run("kline test", func(t *testing.T) {
		s.Subscribe(types.KLineChannel, "BTCUSDT", types.SubscribeOptions{
			Interval: types.Interval30m,
			Depth:    "",
			Speed:    "",
		})
		s.SetPublicOnly()
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnKLine(func(kline types.KLine) {
			t.Log(kline)
		})
		c := make(chan struct{})
		<-c
	})

	t.Run("trade test", func(t *testing.T) {
		err := s.Connect(context.Background())
		assert.NoError(t, err)

		s.OnTradeUpdate(func(trade types.Trade) {
			t.Log("got update", trade.Fee, trade.FeeCurrency, trade)
		})
		c := make(chan struct{})
		<-c
	})
}

func TestStream_parseWebSocketEvent(t *testing.T) {
	s := Stream{}

	t.Run("op", func(t *testing.T) {
		input := `{
		   "success":true,
		   "ret_msg":"subscribe",
		   "conn_id":"a403c8e5-e2b6-4edd-a8f0-1a64fa7227a5",
		   "op":"subscribe"
		}`
		res, err := s.parseWebSocketEvent([]byte(input))
		assert.NoError(t, err)
		opEvent, ok := res.(*WebSocketOpEvent)
		assert.True(t, ok)
		expSucceeds := true
		expRetMsg := "subscribe"
		assert.Equal(t, WebSocketOpEvent{
			Success: expSucceeds,
			RetMsg:  expRetMsg,
			ReqId:   "",
			ConnId:  "a403c8e5-e2b6-4edd-a8f0-1a64fa7227a5",
			Op:      WsOpTypeSubscribe,
			Args:    nil,
		}, *opEvent)
	})
	t.Run("TopicTypeOrderBook with delta", func(t *testing.T) {
		input := `{
			   "topic":"orderbook.50.BTCUSDT",
			   "ts":1691130685111,
			   "type":"delta",
			   "data":{
			      "s":"BTCUSDT",
			      "b":[

			      ],
			      "a":[
			         [
			            "29239.37",
			            "0.082356"
			         ],
			         [
			            "29236.1",
			            "0"
			         ]
			      ],
			      "u":1854104,
			      "seq":10559247733
			   }
			}`

		res, err := s.parseWebSocketEvent([]byte(input))
		assert.NoError(t, err)
		book, ok := res.(*BookEvent)
		assert.True(t, ok)
		assert.Equal(t, BookEvent{
			Symbol: "BTCUSDT",
			Bids:   nil,
			Asks: types.PriceVolumeSlice{
				{
					fixedpoint.NewFromFloat(29239.37),
					fixedpoint.NewFromFloat(0.082356),
				},
				{
					fixedpoint.NewFromFloat(29236.1),
					fixedpoint.NewFromFloat(0),
				},
			},
			UpdateId:   fixedpoint.NewFromFloat(1854104),
			SequenceId: fixedpoint.NewFromFloat(10559247733),
			Type:       DataTypeDelta,
			ServerTime: types.NewMillisecondTimestampFromInt(1691130685111).Time(),
		}, *book)
	})

	t.Run("TopicTypeMarketTrade with snapshot", func(t *testing.T) {
		input := `{
   "topic":"publicTrade.BTCUSDT",
   "ts":1694348711526,
   "type":"snapshot",
   "data":[
      {
         "i":"2290000000068683805",
         "T":1694348711524,
         "p":"25816.27",
         "v":"0.000083",
         "S":"Sell",
         "s":"BTCUSDT",
         "BT":false
      }
   ]
}`

		res, err := s.parseWebSocketEvent([]byte(input))
		assert.NoError(t, err)
		book, ok := res.([]MarketTradeEvent)
		assert.True(t, ok)
		assert.Equal(t, []MarketTradeEvent{
			{
				Timestamp:  types.NewMillisecondTimestampFromInt(1694348711524),
				Symbol:     "BTCUSDT",
				Side:       bybitapi.SideSell,
				Quantity:   fixedpoint.NewFromFloat(0.000083),
				Price:      fixedpoint.NewFromFloat(25816.27),
				Direction:  "",
				TradeId:    "2290000000068683805",
				BlockTrade: false,
			},
		}, book)
	})

	t.Run("TopicTypeKLine with snapshot", func(t *testing.T) {
		input := `{
    "topic": "kline.5.BTCUSDT",
    "data": [
        {
            "start": 1672324800000,
            "end": 1672325099999,
            "interval": "5",
            "open": "16649.5",
            "close": "16677",
            "high": "16677",
            "low": "16608",
            "volume": "2.081",
            "turnover": "34666.4005",
            "confirm": false,
            "timestamp": 1672324988882
        }
    ],
    "ts": 1672324988882,
    "type": "snapshot"
}`

		res, err := s.parseWebSocketEvent([]byte(input))
		assert.NoError(t, err)
		book, ok := res.(*KLineEvent)
		assert.True(t, ok)
		assert.Equal(t, KLineEvent{
			Symbol: "BTCUSDT",
			Type:   DataTypeSnapshot,
			KLines: []KLine{
				{
					StartTime:  types.NewMillisecondTimestampFromInt(1672324800000),
					EndTime:    types.NewMillisecondTimestampFromInt(1672325099999),
					Interval:   "5",
					OpenPrice:  fixedpoint.NewFromFloat(16649.5),
					ClosePrice: fixedpoint.NewFromFloat(16677),
					HighPrice:  fixedpoint.NewFromFloat(16677),
					LowPrice:   fixedpoint.NewFromFloat(16608),
					Volume:     fixedpoint.NewFromFloat(2.081),
					Turnover:   fixedpoint.NewFromFloat(34666.4005),
					Confirm:    false,
					Timestamp:  types.NewMillisecondTimestampFromInt(1672324988882),
				},
			},
		}, *book)
	})

	t.Run("TopicTypeKLine with invalid topic", func(t *testing.T) {
		input := `{
    "topic": "kline.5",
    "data": [
        {
            "start": 1672324800000,
            "end": 1672325099999,
            "interval": "5",
            "open": "16649.5",
            "close": "16677",
            "high": "16677",
            "low": "16608",
            "volume": "2.081",
            "turnover": "34666.4005",
            "confirm": false,
            "timestamp": 1672324988882
        }
    ],
    "ts": 1672324988882,
    "type": "snapshot"
}`

		res, err := s.parseWebSocketEvent([]byte(input))
		assert.Equal(t, errors.New("unexpected topic: kline.5"), err)
		assert.Nil(t, res)
	})

	t.Run("Parse fails", func(t *testing.T) {
		input := `{
			   "topic":"orderbook.50.BTCUSDT",
			   "ts":1691130685111,
			   "type":"delta",
			   "data":{
					"GG": "test",
			   }
			}`

		res, err := s.parseWebSocketEvent([]byte(input))
		assert.Error(t, err)
		assert.Equal(t, nil, res)
	})
}

func Test_convertSubscription(t *testing.T) {
	s := Stream{}
	t.Run("BookChannel.DepthLevel1", func(t *testing.T) {
		res, err := s.convertSubscription(types.Subscription{
			Symbol:  "BTCUSDT",
			Channel: types.BookChannel,
			Options: types.SubscribeOptions{
				Depth: types.DepthLevel1,
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, genTopic(TopicTypeOrderBook, types.DepthLevel1, "BTCUSDT"), res)
	})
	t.Run("BookChannel.DepthLevel50", func(t *testing.T) {
		res, err := s.convertSubscription(types.Subscription{
			Symbol:  "BTCUSDT",
			Channel: types.BookChannel,
			Options: types.SubscribeOptions{
				Depth: types.DepthLevel50,
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, genTopic(TopicTypeOrderBook, types.DepthLevel50, "BTCUSDT"), res)
	})
	t.Run("BookChannel.DepthLevel200", func(t *testing.T) {
		res, err := s.convertSubscription(types.Subscription{
			Symbol:  "BTCUSDT",
			Channel: types.BookChannel,
			Options: types.SubscribeOptions{
				Depth: types.DepthLevel200,
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, genTopic(TopicTypeOrderBook, types.DepthLevel200, "BTCUSDT"), res)
	})
	t.Run("BookChannel. with default depth", func(t *testing.T) {
		res, err := s.convertSubscription(types.Subscription{
			Symbol:  "BTCUSDT",
			Channel: types.BookChannel,
		})
		assert.NoError(t, err)
		assert.Equal(t, genTopic(TopicTypeOrderBook, types.DepthLevel1, "BTCUSDT"), res)
	})
	t.Run("BookChannel.DepthLevel50", func(t *testing.T) {
		res, err := s.convertSubscription(types.Subscription{
			Symbol:  "BTCUSDT",
			Channel: types.BookChannel,
			Options: types.SubscribeOptions{
				Depth: types.DepthLevel50,
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, genTopic(TopicTypeOrderBook, types.DepthLevel50, "BTCUSDT"), res)
	})
	t.Run("BookChannel. not support depth, use default level 1", func(t *testing.T) {
		res, err := s.convertSubscription(types.Subscription{
			Symbol:  "BTCUSDT",
			Channel: types.BookChannel,
			Options: types.SubscribeOptions{
				Depth: "20",
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, genTopic(TopicTypeOrderBook, types.DepthLevel1, "BTCUSDT"), res)
	})

	t.Run("unsupported channel", func(t *testing.T) {
		res, err := s.convertSubscription(types.Subscription{
			Symbol:  "BTCUSDT",
			Channel: "unsupported",
		})
		assert.Equal(t, fmt.Errorf("unsupported stream channel: %s", "unsupported"), err)
		assert.Equal(t, "", res)
	})

	t.Run("MarketTradeChannel", func(t *testing.T) {
		res, err := s.convertSubscription(types.Subscription{
			Symbol:  "BTCUSDT",
			Channel: types.MarketTradeChannel,
			Options: types.SubscribeOptions{},
		})
		assert.NoError(t, err)
		assert.Equal(t, genTopic(TopicTypeMarketTrade, "BTCUSDT"), res)
	})
}

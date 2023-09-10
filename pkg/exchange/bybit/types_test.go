package bybit

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_parseWebSocketEvent(t *testing.T) {
	t.Run("[public] PingEvent without req id", func(t *testing.T) {
		s := NewStream("", "", nil)
		msg := `{"success":true,"ret_msg":"pong","conn_id":"a806f6c4-3608-4b6d-a225-9f5da975bc44","op":"ping"}`
		raw, err := s.parseWebSocketEvent([]byte(msg))
		assert.NoError(t, err)

		expRetMsg := string(WsOpTypePong)
		e, ok := raw.(*WebSocketOpEvent)
		assert.True(t, ok)
		assert.Equal(t, &WebSocketOpEvent{
			Success: true,
			RetMsg:  expRetMsg,
			ConnId:  "a806f6c4-3608-4b6d-a225-9f5da975bc44",
			ReqId:   "",
			Op:      WsOpTypePing,
			Args:    nil,
		}, e)

		assert.NoError(t, e.IsValid())
	})

	t.Run("[public] PingEvent with req id", func(t *testing.T) {
		s := NewStream("", "", nil)
		msg := `{"success":true,"ret_msg":"pong","conn_id":"a806f6c4-3608-4b6d-a225-9f5da975bc44","req_id":"b26704da-f5af-44c2-bdf7-935d6739e1a0","op":"ping"}`
		raw, err := s.parseWebSocketEvent([]byte(msg))
		assert.NoError(t, err)

		expRetMsg := string(WsOpTypePong)
		expReqId := "b26704da-f5af-44c2-bdf7-935d6739e1a0"
		e, ok := raw.(*WebSocketOpEvent)
		assert.True(t, ok)
		assert.Equal(t, &WebSocketOpEvent{
			Success: true,
			RetMsg:  expRetMsg,
			ConnId:  "a806f6c4-3608-4b6d-a225-9f5da975bc44",
			ReqId:   expReqId,
			Op:      WsOpTypePing,
			Args:    nil,
		}, e)

		assert.NoError(t, e.IsValid())
	})

	t.Run("[private] PingEvent without req id", func(t *testing.T) {
		s := NewStream("", "", nil)
		msg := `{"op":"pong","args":["1690884539181"],"conn_id":"civn4p1dcjmtvb69ome0-yrt1"}`
		raw, err := s.parseWebSocketEvent([]byte(msg))
		assert.NoError(t, err)

		e, ok := raw.(*WebSocketOpEvent)
		assert.True(t, ok)
		assert.Equal(t, &WebSocketOpEvent{
			Success: false,
			RetMsg:  "",
			ConnId:  "civn4p1dcjmtvb69ome0-yrt1",
			ReqId:   "",
			Op:      WsOpTypePong,
			Args:    []string{"1690884539181"},
		}, e)

		assert.NoError(t, e.IsValid())
	})

	t.Run("[private] PingEvent with req id", func(t *testing.T) {
		s := NewStream("", "", nil)
		msg := `{"req_id":"78d36b57-a142-47b7-9143-5843df77d44d","op":"pong","args":["1690884539181"],"conn_id":"civn4p1dcjmtvb69ome0-yrt1"}`
		raw, err := s.parseWebSocketEvent([]byte(msg))
		assert.NoError(t, err)

		expReqId := "78d36b57-a142-47b7-9143-5843df77d44d"
		e, ok := raw.(*WebSocketOpEvent)
		assert.True(t, ok)
		assert.Equal(t, &WebSocketOpEvent{
			Success: false,
			RetMsg:  "",
			ConnId:  "civn4p1dcjmtvb69ome0-yrt1",
			ReqId:   expReqId,
			Op:      WsOpTypePong,
			Args:    []string{"1690884539181"},
		}, e)

		assert.NoError(t, e.IsValid())
	})
}

func Test_WebSocketEventIsValid(t *testing.T) {
	t.Run("[public] valid op ping", func(t *testing.T) {
		expRetMsg := string(WsOpTypePong)
		expReqId := "b26704da-f5af-44c2-bdf7-935d6739e1a0"

		w := &WebSocketOpEvent{
			Success: true,
			RetMsg:  expRetMsg,
			ReqId:   expReqId,
			ConnId:  "test-conndid",
			Op:      WsOpTypePing,
			Args:    nil,
		}
		assert.NoError(t, w.IsValid())
	})

	t.Run("[private] valid op ping", func(t *testing.T) {
		w := &WebSocketOpEvent{
			Success: false,
			RetMsg:  "",
			ReqId:   "",
			ConnId:  "test-conndid",
			Op:      WsOpTypePong,
			Args:    nil,
		}
		assert.NoError(t, w.IsValid())
	})

	t.Run("[public] un-Success", func(t *testing.T) {
		expRetMsg := string(WsOpTypePong)
		expReqId := "b26704da-f5af-44c2-bdf7-935d6739e1a0"

		w := &WebSocketOpEvent{
			Success: false,
			RetMsg:  expRetMsg,
			ReqId:   expReqId,
			ConnId:  "test-conndid",
			Op:      WsOpTypePing,
			Args:    nil,
		}
		assert.Equal(t, fmt.Errorf("unexpected response result: %+v", w), w.IsValid())
	})

	t.Run("[public] invalid ret msg", func(t *testing.T) {
		expRetMsg := "PINGPONGPINGPONG"
		expReqId := "b26704da-f5af-44c2-bdf7-935d6739e1a0"

		w := &WebSocketOpEvent{
			Success: false,
			RetMsg:  expRetMsg,
			ReqId:   expReqId,
			ConnId:  "test-conndid",
			Op:      WsOpTypePing,
			Args:    nil,
		}
		assert.Equal(t, fmt.Errorf("unexpected response result: %+v", w), w.IsValid())
	})

	t.Run("[public] missing RetMsg field", func(t *testing.T) {
		expReqId := "b26704da-f5af-44c2-bdf7-935d6739e1a0"

		w := &WebSocketOpEvent{
			ReqId:  expReqId,
			ConnId: "test-conndid",
			Op:     WsOpTypePing,
			Args:   nil,
		}
		assert.Equal(t, fmt.Errorf("unexpected response result: %+v", w), w.IsValid())
	})

	t.Run("unexpected op type", func(t *testing.T) {
		w := &WebSocketOpEvent{
			Op: WsOpType("unexpected"),
		}
		assert.Equal(t, fmt.Errorf("unexpected op type: %+v", w), w.IsValid())
	})

	t.Run("[subscribe] valid with public channel", func(t *testing.T) {
		expRetMsg := "subscribe"
		w := &WebSocketOpEvent{
			Success: true,
			RetMsg:  expRetMsg,
			ReqId:   "",
			ConnId:  "test-conndid",
			Op:      WsOpTypeSubscribe,
			Args:    nil,
		}
		assert.NoError(t, w.IsValid())
	})

	t.Run("[unsubscribe] valid with public channel", func(t *testing.T) {
		expRetMsg := "subscribe"
		w := &WebSocketOpEvent{
			Success: true,
			RetMsg:  expRetMsg,
			ReqId:   "",
			ConnId:  "test-conndid",
			Op:      WsOpTypeUnsubscribe,
			Args:    nil,
		}
		assert.NoError(t, w.IsValid())
	})

	t.Run("[subscribe] valid with private channel", func(t *testing.T) {
		w := &WebSocketOpEvent{
			Success: true,
			RetMsg:  "",
			ReqId:   "",
			ConnId:  "test-conndid",
			Op:      WsOpTypeSubscribe,
			Args:    nil,
		}
		assert.NoError(t, w.IsValid())
	})

	t.Run("[subscribe] un-succeeds", func(t *testing.T) {
		expRetMsg := ""
		w := &WebSocketOpEvent{
			Success: false,
			RetMsg:  expRetMsg,
			ReqId:   "",
			ConnId:  "test-conndid",
			Op:      WsOpTypeSubscribe,
			Args:    nil,
		}
		assert.Equal(t, fmt.Errorf("unexpected response result: %+v", w), w.IsValid())
	})

	t.Run("[unsubscribe] un-succeeds", func(t *testing.T) {
		expRetMsg := ""
		w := &WebSocketOpEvent{
			Success: false,
			RetMsg:  expRetMsg,
			ReqId:   "",
			ConnId:  "test-conndid",
			Op:      WsOpTypeUnsubscribe,
			Args:    nil,
		}
		assert.Equal(t, fmt.Errorf("unexpected response result: %+v", w), w.IsValid())
	})

	t.Run("[auth] valid", func(t *testing.T) {
		w := &WebSocketOpEvent{
			Success: true,
			RetMsg:  "",
			ReqId:   "",
			ConnId:  "test-conndid",
			Op:      WsOpTypeAuth,
			Args:    nil,
		}
		assert.NoError(t, w.IsValid())
	})

	t.Run("[subscribe] un-succeeds", func(t *testing.T) {
		expRetMsg := "invalid signature"
		w := &WebSocketOpEvent{
			Success: false,
			RetMsg:  expRetMsg,
			ReqId:   "",
			ConnId:  "test-conndid",
			Op:      WsOpTypeAuth,
			Args:    nil,
		}
		assert.Equal(t, fmt.Errorf("unexpected response result: %+v", w), w.IsValid())
	})
}

func TestBookEvent_OrderBook(t *testing.T) {
	t.Run("snapshot", func(t *testing.T) {
		/*
			{
			   "topic":"orderbook.50.BTCUSDT",
			   "ts":1691129753071,
			   "type":"snapshot",
			   "data":{
			      "s":"BTCUSDT",
			      "b":[
			         [
			            "29230.81",
			            "4.713817"
			         ],
			         [
			            "29230",
			            "0.1646"
			         ],
			         [
			            "29229.92",
			            "0.036"
			         ],
			      ],
			      "a":[
			         [
			            "29230.82",
			            "2.745421"
			         ],
			         [
			            "29231.41",
			            "1.6"
			         ],
			         [
			            "29231.42",
			            "0.513654"
			         ],
			      ],
			      "u":1841364,
			      "seq":10558648910
			   }
			}
		*/
		event := &BookEvent{
			Symbol: "BTCUSDT",
			Bids: types.PriceVolumeSlice{
				{
					fixedpoint.NewFromFloat(29230.81),
					fixedpoint.NewFromFloat(4.713817),
				},
				{
					fixedpoint.NewFromFloat(29230),
					fixedpoint.NewFromFloat(0.1646),
				},
				{
					fixedpoint.NewFromFloat(29229.92),
					fixedpoint.NewFromFloat(0.036),
				},
			},
			Asks: types.PriceVolumeSlice{
				{
					fixedpoint.NewFromFloat(29230.82),
					fixedpoint.NewFromFloat(2.745421),
				},
				{
					fixedpoint.NewFromFloat(29231.41),
					fixedpoint.NewFromFloat(1.6),
				},
				{
					fixedpoint.NewFromFloat(29231.42),
					fixedpoint.NewFromFloat(0.513654),
				},
			},
			UpdateId:   fixedpoint.NewFromFloat(1841364),
			SequenceId: fixedpoint.NewFromFloat(10558648910),
			Type:       DataTypeSnapshot,
		}

		expSliceOrderBook := types.SliceOrderBook{
			Symbol: event.Symbol,
			Bids:   event.Bids,
			Asks:   event.Asks,
		}

		assert.Equal(t, expSliceOrderBook, event.OrderBook())
	})
	t.Run("delta", func(t *testing.T) {
		/*
			{
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
			}
		*/
		event := &BookEvent{
			Symbol: "BTCUSDT",
			Bids:   types.PriceVolumeSlice{},
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
		}

		expSliceOrderBook := types.SliceOrderBook{
			Symbol: event.Symbol,
			Bids:   types.PriceVolumeSlice{},
			Asks:   event.Asks,
		}

		assert.Equal(t, expSliceOrderBook, event.OrderBook())
	})

}

func TestMarketTradeEvent_Trade(t *testing.T) {
	qty := fixedpoint.NewFromFloat(0.002289)
	price := fixedpoint.NewFromFloat(28829.7600)
	tradeId := uint64(2290000000068683542)
	tradeTime := types.NewMillisecondTimestampFromInt(1691486100000)
	event := MarketTradeEvent{
		Timestamp:  tradeTime,
		Symbol:     "BTCUSDT",
		Side:       bybitapi.SideSell,
		Quantity:   qty,
		Price:      price,
		Direction:  "",
		TradeId:    strconv.FormatUint(tradeId, 10),
		BlockTrade: false,
	}
	t.Run("succeeds", func(t *testing.T) {
		expEvent := types.Trade{
			ID:            tradeId,
			Exchange:      types.ExchangeBybit,
			Price:         price,
			Quantity:      qty,
			QuoteQuantity: price.Mul(qty),
			Symbol:        event.Symbol,
			Side:          types.SideTypeSell,
			IsBuyer:       false,
			IsMaker:       false,
			Time:          types.Time(tradeTime.Time()),
		}

		trade, err := event.toGlobalTrade()
		assert.NoError(t, err)
		assert.Equal(t, expEvent, trade)
	})

	t.Run("invalid side", func(t *testing.T) {
		newEvent := event
		newEvent.Side = "invalid"
		_, err := newEvent.toGlobalTrade()
		assert.ErrorContains(t, err, "unexpected side")
	})

	t.Run("invalid trade id", func(t *testing.T) {
		newEvent := event
		newEvent.TradeId = "invalid"
		_, err := newEvent.toGlobalTrade()
		assert.ErrorContains(t, err, "unexpected trade id")
	})
}

func Test_genTopicName(t *testing.T) {
	exp := "orderbook.50.BTCUSDT"
	assert.Equal(t, exp, genTopic(TopicTypeOrderBook, types.DepthLevel50, "BTCUSDT"))
}

func Test_getTopicName(t *testing.T) {
	exp := TopicTypeOrderBook
	assert.Equal(t, exp, getTopicType("orderbook.50.BTCUSDT"))
}

func Test_getSymbolFromTopic(t *testing.T) {
	t.Run("succeeds", func(t *testing.T) {
		exp := "BTCUSDT"
		res, err := getSymbolFromTopic("kline.1.BTCUSDT")
		assert.NoError(t, err)
		assert.Equal(t, exp, res)
	})

	t.Run("unexpected topic", func(t *testing.T) {
		res, err := getSymbolFromTopic("kline.1")
		assert.Empty(t, res)
		assert.Equal(t, err, fmt.Errorf("unexpected topic: kline.1"))
	})
}

func TestKLine_toGlobalKLine(t *testing.T) {
	t.Run("succeeds", func(t *testing.T) {
		k := KLine{
			StartTime:  types.NewMillisecondTimestampFromInt(1691486100000),
			EndTime:    types.NewMillisecondTimestampFromInt(1691487000000),
			Interval:   "1",
			OpenPrice:  fixedpoint.NewFromFloat(29045.3),
			ClosePrice: fixedpoint.NewFromFloat(29228.56),
			HighPrice:  fixedpoint.NewFromFloat(29228.56),
			LowPrice:   fixedpoint.NewFromFloat(29045.3),
			Volume:     fixedpoint.NewFromFloat(9.265593),
			Turnover:   fixedpoint.NewFromFloat(270447.43520753),
			Confirm:    false,
			Timestamp:  types.NewMillisecondTimestampFromInt(1691486100000),
		}

		gKline, err := k.toGlobalKLine("BTCUSDT")
		assert.NoError(t, err)

		assert.Equal(t, types.KLine{
			Exchange:    types.ExchangeBybit,
			Symbol:      "BTCUSDT",
			StartTime:   types.Time(k.StartTime.Time()),
			EndTime:     types.Time(k.EndTime.Time()),
			Interval:    types.Interval1m,
			Open:        fixedpoint.NewFromFloat(29045.3),
			Close:       fixedpoint.NewFromFloat(29228.56),
			High:        fixedpoint.NewFromFloat(29228.56),
			Low:         fixedpoint.NewFromFloat(29045.3),
			Volume:      fixedpoint.NewFromFloat(9.265593),
			QuoteVolume: fixedpoint.NewFromFloat(270447.43520753),
			Closed:      false,
		}, gKline)
	})

	t.Run("interval not supported", func(t *testing.T) {
		k := KLine{
			StartTime:  types.NewMillisecondTimestampFromInt(1691486100000),
			EndTime:    types.NewMillisecondTimestampFromInt(1691487000000),
			Interval:   "112",
			OpenPrice:  fixedpoint.NewFromFloat(29045.3),
			ClosePrice: fixedpoint.NewFromFloat(29228.56),
			HighPrice:  fixedpoint.NewFromFloat(29228.56),
			LowPrice:   fixedpoint.NewFromFloat(29045.3),
			Volume:     fixedpoint.NewFromFloat(9.265593),
			Turnover:   fixedpoint.NewFromFloat(270447.43520753),
			Confirm:    false,
			Timestamp:  types.NewMillisecondTimestampFromInt(1691486100000),
		}

		gKline, err := k.toGlobalKLine("BTCUSDT")
		assert.Equal(t, fmt.Errorf("unexpected k line interval: %+v", &k), err)
		assert.Equal(t, gKline, types.KLine{})
	})
}

func TestTradeEvent_toGlobalTrade(t *testing.T) {
	/*
		{
		   "category":"spot",
		   "symbol":"BTCUSDT",
		   "execFee":"",
		   "execId":"2100000000032905730",
		   "execPrice":"28829.7600",
		   "execQty":"0.002289",
		   "execType":"",
		   "execValue":"",
		   "isMaker":false,
		   "feeRate":"",
		   "tradeIv":"",
		   "markIv":"",
		   "blockTradeId":"",
		   "markPrice":"",
		   "indexPrice":"",
		   "underlyingPrice":"",
		   "leavesQty":"",
		   "orderId":"1482125285219500288",
		   "orderLinkId":"1691419101980",
		   "orderPrice":"",
		   "orderQty":"",
		   "orderType":"",
		   "stopOrderType":"",
		   "side":"Buy",
		   "execTime":"1691419102282",
		   "isLeverage":"0"
		}
	*/
	t.Run("succeeds", func(t *testing.T) {
		symbolFee := symbolFeeDetail{
			FeeRate: bybitapi.FeeRate{
				Symbol:       "BTCUSDT",
				TakerFeeRate: fixedpoint.NewFromFloat(0.001),
				MakerFeeRate: fixedpoint.NewFromFloat(0.002),
			},
			BaseCoin:  "BTC",
			QuoteCoin: "USDT",
		}
		qty := fixedpoint.NewFromFloat(0.002289)
		price := fixedpoint.NewFromFloat(28829.7600)
		timeNow := time.Now().Truncate(time.Second)
		expTrade := &types.Trade{
			ID:            2100000000032905730,
			OrderID:       1482125285219500288,
			Exchange:      types.ExchangeBybit,
			Price:         price,
			Quantity:      qty,
			QuoteQuantity: qty.Mul(price),
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeBuy,
			IsBuyer:       true,
			IsMaker:       false,
			Time:          types.Time(timeNow),
			Fee:           symbolFee.FeeRate.TakerFeeRate.Mul(qty),
			FeeCurrency:   "BTC",
		}
		tradeEvent := TradeEvent{
			OrderId:         fmt.Sprintf("%d", expTrade.OrderID),
			OrderLinkId:     "1691419101980",
			Category:        "spot",
			Symbol:          expTrade.Symbol,
			ExecId:          fmt.Sprintf("%d", expTrade.ID),
			ExecPrice:       expTrade.Price,
			ExecQty:         expTrade.Quantity,
			IsMaker:         false,
			BlockTradeId:    "",
			OrderType:       "",
			Side:            bybitapi.SideBuy,
			ExecTime:        types.MillisecondTimestamp(timeNow),
			ClosedSize:      fixedpoint.NewFromInt(0),
			ExecFee:         fixedpoint.NewFromInt(0),
			ExecType:        "",
			ExecValue:       fixedpoint.NewFromInt(0),
			FeeRate:         fixedpoint.NewFromInt(0),
			LeavesQty:       fixedpoint.NewFromInt(0),
			OrderPrice:      fixedpoint.NewFromInt(0),
			OrderQty:        fixedpoint.NewFromInt(0),
			StopOrderType:   "",
			IsLeverage:      "0",
			MarkIv:          "",
			MarkPrice:       fixedpoint.NewFromInt(0),
			IndexPrice:      fixedpoint.NewFromInt(0),
			UnderlyingPrice: fixedpoint.NewFromInt(0),
			TradeIv:         "",
		}

		actualTrade, err := tradeEvent.toGlobalTrade(symbolFee)
		assert.NoError(t, err)
		assert.Equal(t, expTrade, actualTrade)
	})

	t.Run("unexpected category", func(t *testing.T) {
		tradeEvent := TradeEvent{
			Category: "test-spot",
		}

		actualTrade, err := tradeEvent.toGlobalTrade(symbolFeeDetail{})
		assert.Equal(t, fmt.Errorf("unexected category: %s", tradeEvent.Category), err)
		assert.Nil(t, actualTrade)
	})

	t.Run("unexpected side", func(t *testing.T) {
		tradeEvent := TradeEvent{
			Category: "spot",
			Side:     bybitapi.Side("BOTH"),
		}

		actualTrade, err := tradeEvent.toGlobalTrade(symbolFeeDetail{})
		assert.Equal(t, fmt.Errorf("unexpected side: BOTH"), err)
		assert.Nil(t, actualTrade)
	})

	t.Run("unexpected order id", func(t *testing.T) {
		tradeEvent := TradeEvent{
			Category: "spot",
			Side:     bybitapi.SideBuy,
			OrderId:  "ABCD3123",
		}

		_, nerr := strconv.ParseUint(tradeEvent.OrderId, 10, 64)
		actualTrade, err := tradeEvent.toGlobalTrade(symbolFeeDetail{})
		assert.Equal(t, fmt.Errorf("unexpected order id: %s, err: %w", tradeEvent.OrderId, nerr), err)
		assert.Nil(t, actualTrade)
	})

	t.Run("unexpected exec id", func(t *testing.T) {
		tradeEvent := TradeEvent{
			Category: "spot",
			Side:     bybitapi.SideBuy,
			OrderId:  "3123",
			ExecId:   "ABC3123",
		}

		_, nerr := strconv.ParseUint(tradeEvent.ExecId, 10, 64)
		actualTrade, err := tradeEvent.toGlobalTrade(symbolFeeDetail{})
		assert.Equal(t, fmt.Errorf("unexpected exec id: %s, err: %w", tradeEvent.ExecId, nerr), err)
		assert.Nil(t, actualTrade)
	})
}

func TestTradeEvent_CalculateFee(t *testing.T) {
	t.Run("maker fee positive, maker, buyer", func(t *testing.T) {
		symbolFee := symbolFeeDetail{
			FeeRate: bybitapi.FeeRate{
				Symbol:       "BTCUSDT",
				TakerFeeRate: fixedpoint.NewFromFloat(0.001),
				MakerFeeRate: fixedpoint.NewFromFloat(0.002),
			},
			BaseCoin:  "BTC",
			QuoteCoin: "USDT",
		}

		qty := fixedpoint.NewFromFloat(0.010000)
		price := fixedpoint.NewFromFloat(28830.8100)
		trade := &TradeEvent{
			ExecPrice: price,
			ExecQty:   qty,
			IsMaker:   true,
			Side:      bybitapi.SideBuy,
		}

		feeCurrency, fee := calculateFee(*trade, symbolFee)
		assert.Equal(t, feeCurrency, "BTC")
		assert.Equal(t, fee, qty.Mul(symbolFee.FeeRate.MakerFeeRate))
	})

	t.Run("maker fee positive, maker, seller", func(t *testing.T) {
		symbolFee := symbolFeeDetail{
			FeeRate: bybitapi.FeeRate{
				Symbol:       "BTCUSDT",
				TakerFeeRate: fixedpoint.NewFromFloat(0.001),
				MakerFeeRate: fixedpoint.NewFromFloat(0.002),
			},
			BaseCoin:  "BTC",
			QuoteCoin: "USDT",
		}

		qty := fixedpoint.NewFromFloat(0.010000)
		price := fixedpoint.NewFromFloat(28830.8099)
		trade := &TradeEvent{
			ExecPrice: price,
			ExecQty:   qty,
			IsMaker:   true,
			Side:      bybitapi.SideSell,
		}

		feeCurrency, fee := calculateFee(*trade, symbolFee)
		assert.Equal(t, feeCurrency, "USDT")
		assert.Equal(t, fee, qty.Mul(price).Mul(symbolFee.FeeRate.MakerFeeRate))
	})

	t.Run("maker fee positive, taker, buyer", func(t *testing.T) {
		symbolFee := symbolFeeDetail{
			FeeRate: bybitapi.FeeRate{
				Symbol:       "BTCUSDT",
				TakerFeeRate: fixedpoint.NewFromFloat(0.001),
				MakerFeeRate: fixedpoint.NewFromFloat(0.002),
			},
			BaseCoin:  "BTC",
			QuoteCoin: "USDT",
		}

		qty := fixedpoint.NewFromFloat(0.010000)
		price := fixedpoint.NewFromFloat(28830.8100)
		trade := &TradeEvent{
			ExecPrice: price,
			ExecQty:   qty,
			IsMaker:   false,
			Side:      bybitapi.SideBuy,
		}

		feeCurrency, fee := calculateFee(*trade, symbolFee)
		assert.Equal(t, feeCurrency, "BTC")
		assert.Equal(t, fee, qty.Mul(symbolFee.FeeRate.TakerFeeRate))
	})

	t.Run("maker fee positive, taker, seller", func(t *testing.T) {
		symbolFee := symbolFeeDetail{
			FeeRate: bybitapi.FeeRate{
				Symbol:       "BTCUSDT",
				TakerFeeRate: fixedpoint.NewFromFloat(0.001),
				MakerFeeRate: fixedpoint.NewFromFloat(0.002),
			},
			BaseCoin:  "BTC",
			QuoteCoin: "USDT",
		}

		qty := fixedpoint.NewFromFloat(0.010000)
		price := fixedpoint.NewFromFloat(28830.8099)
		trade := &TradeEvent{
			ExecPrice: price,
			ExecQty:   qty,
			IsMaker:   false,
			Side:      bybitapi.SideSell,
		}

		feeCurrency, fee := calculateFee(*trade, symbolFee)
		assert.Equal(t, feeCurrency, "USDT")
		assert.Equal(t, fee, qty.Mul(price).Mul(symbolFee.FeeRate.TakerFeeRate))
	})

	t.Run("maker fee negative, maker, buyer", func(t *testing.T) {
		symbolFee := symbolFeeDetail{
			FeeRate: bybitapi.FeeRate{
				Symbol:       "BTCUSDT",
				TakerFeeRate: fixedpoint.NewFromFloat(-0.001),
				MakerFeeRate: fixedpoint.NewFromFloat(-0.002),
			},
			BaseCoin:  "BTC",
			QuoteCoin: "USDT",
		}

		qty := fixedpoint.NewFromFloat(0.002289)
		price := fixedpoint.NewFromFloat(28829.7600)
		trade := &TradeEvent{
			ExecPrice: price,
			ExecQty:   qty,
			IsMaker:   true,
			Side:      bybitapi.SideBuy,
		}

		feeCurrency, fee := calculateFee(*trade, symbolFee)
		assert.Equal(t, feeCurrency, "USDT")
		assert.Equal(t, fee, qty.Mul(price).Mul(symbolFee.FeeRate.MakerFeeRate))
	})

	t.Run("maker fee negative, maker, seller", func(t *testing.T) {
		symbolFee := symbolFeeDetail{
			FeeRate: bybitapi.FeeRate{
				Symbol:       "BTCUSDT",
				TakerFeeRate: fixedpoint.NewFromFloat(-0.001),
				MakerFeeRate: fixedpoint.NewFromFloat(-0.002),
			},
			BaseCoin:  "BTC",
			QuoteCoin: "USDT",
		}

		qty := fixedpoint.NewFromFloat(0.002289)
		price := fixedpoint.NewFromFloat(28829.7600)
		trade := &TradeEvent{
			ExecPrice: price,
			ExecQty:   qty,
			IsMaker:   true,
			Side:      bybitapi.SideSell,
		}

		feeCurrency, fee := calculateFee(*trade, symbolFee)
		assert.Equal(t, feeCurrency, "BTC")
		assert.Equal(t, fee, qty.Mul(symbolFee.FeeRate.MakerFeeRate))
	})

	t.Run("maker fee negative, taker, buyer", func(t *testing.T) {
		symbolFee := symbolFeeDetail{
			FeeRate: bybitapi.FeeRate{
				Symbol:       "BTCUSDT",
				TakerFeeRate: fixedpoint.NewFromFloat(-0.001),
				MakerFeeRate: fixedpoint.NewFromFloat(-0.002),
			},
			BaseCoin:  "BTC",
			QuoteCoin: "USDT",
		}

		qty := fixedpoint.NewFromFloat(0.002289)
		price := fixedpoint.NewFromFloat(28829.7600)
		trade := &TradeEvent{
			ExecPrice: price,
			ExecQty:   qty,
			IsMaker:   false,
			Side:      bybitapi.SideBuy,
		}

		feeCurrency, fee := calculateFee(*trade, symbolFee)
		assert.Equal(t, feeCurrency, "BTC")
		assert.Equal(t, fee, qty.Mul(symbolFee.FeeRate.TakerFeeRate))
	})

	t.Run("maker fee negative, taker, seller", func(t *testing.T) {
		symbolFee := symbolFeeDetail{
			FeeRate: bybitapi.FeeRate{
				Symbol:       "BTCUSDT",
				TakerFeeRate: fixedpoint.NewFromFloat(-0.001),
				MakerFeeRate: fixedpoint.NewFromFloat(-0.002),
			},
			BaseCoin:  "BTC",
			QuoteCoin: "USDT",
		}

		qty := fixedpoint.NewFromFloat(0.002289)
		price := fixedpoint.NewFromFloat(28829.7600)
		trade := &TradeEvent{
			ExecPrice: price,
			ExecQty:   qty,
			IsMaker:   false,
			Side:      bybitapi.SideSell,
		}

		feeCurrency, fee := calculateFee(*trade, symbolFee)
		assert.Equal(t, feeCurrency, "USDT")
		assert.Equal(t, fee, qty.Mul(price).Mul(symbolFee.FeeRate.TakerFeeRate))
	})

}

func TestTradeEvent_baseCoinAsFee(t *testing.T) {
	symbolFee := symbolFeeDetail{
		FeeRate: bybitapi.FeeRate{
			Symbol:       "BTCUSDT",
			TakerFeeRate: fixedpoint.NewFromFloat(0.001),
			MakerFeeRate: fixedpoint.NewFromFloat(0.002),
		},
		BaseCoin:  "BTC",
		QuoteCoin: "USDT",
	}
	qty := fixedpoint.NewFromFloat(0.002289)
	price := fixedpoint.NewFromFloat(28829.7600)
	trade := &TradeEvent{
		ExecPrice: price,
		ExecQty:   qty,
		IsMaker:   false,
	}
	assert.Equal(t, symbolFee.FeeRate.TakerFeeRate.Mul(qty), baseCoinAsFee(*trade, symbolFee))

	trade.IsMaker = true
	assert.Equal(t, symbolFee.FeeRate.MakerFeeRate.Mul(qty), baseCoinAsFee(*trade, symbolFee))
}

func TestTradeEvent_quoteCoinAsFee(t *testing.T) {
	symbolFee := symbolFeeDetail{
		FeeRate: bybitapi.FeeRate{
			Symbol:       "BTCUSDT",
			TakerFeeRate: fixedpoint.NewFromFloat(0.001),
			MakerFeeRate: fixedpoint.NewFromFloat(0.002),
		},
		BaseCoin:  "BTC",
		QuoteCoin: "USDT",
	}
	qty := fixedpoint.NewFromFloat(0.002289)
	price := fixedpoint.NewFromFloat(28829.7600)
	trade := &TradeEvent{
		ExecPrice: price,
		ExecQty:   qty,
		IsMaker:   false,
	}
	assert.Equal(t, symbolFee.FeeRate.TakerFeeRate.Mul(qty.Mul(price)), quoteCoinAsFee(*trade, symbolFee))

	trade.IsMaker = true
	assert.Equal(t, symbolFee.FeeRate.MakerFeeRate.Mul(qty.Mul(price)), quoteCoinAsFee(*trade, symbolFee))
}

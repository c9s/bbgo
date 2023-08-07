package bybit

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_parseWebSocketEvent(t *testing.T) {
	t.Run("[public] PingEvent without req id", func(t *testing.T) {
		s := NewStream("", "")
		msg := `{"success":true,"ret_msg":"pong","conn_id":"a806f6c4-3608-4b6d-a225-9f5da975bc44","op":"ping"}`
		raw, err := s.parseWebSocketEvent([]byte(msg))
		assert.NoError(t, err)

		expSucceeds := true
		expRetMsg := string(WsOpTypePong)
		e, ok := raw.(*WebSocketOpEvent)
		assert.True(t, ok)
		assert.Equal(t, &WebSocketOpEvent{
			Success: &expSucceeds,
			RetMsg:  &expRetMsg,
			ConnId:  "a806f6c4-3608-4b6d-a225-9f5da975bc44",
			ReqId:   nil,
			Op:      WsOpTypePing,
			Args:    nil,
		}, e)

		assert.NoError(t, e.IsValid())
	})

	t.Run("[public] PingEvent with req id", func(t *testing.T) {
		s := NewStream("", "")
		msg := `{"success":true,"ret_msg":"pong","conn_id":"a806f6c4-3608-4b6d-a225-9f5da975bc44","req_id":"b26704da-f5af-44c2-bdf7-935d6739e1a0","op":"ping"}`
		raw, err := s.parseWebSocketEvent([]byte(msg))
		assert.NoError(t, err)

		expSucceeds := true
		expRetMsg := string(WsOpTypePong)
		expReqId := "b26704da-f5af-44c2-bdf7-935d6739e1a0"
		e, ok := raw.(*WebSocketOpEvent)
		assert.True(t, ok)
		assert.Equal(t, &WebSocketOpEvent{
			Success: &expSucceeds,
			RetMsg:  &expRetMsg,
			ConnId:  "a806f6c4-3608-4b6d-a225-9f5da975bc44",
			ReqId:   &expReqId,
			Op:      WsOpTypePing,
			Args:    nil,
		}, e)

		assert.NoError(t, e.IsValid())
	})

	t.Run("[private] PingEvent without req id", func(t *testing.T) {
		s := NewStream("", "")
		msg := `{"op":"pong","args":["1690884539181"],"conn_id":"civn4p1dcjmtvb69ome0-yrt1"}`
		raw, err := s.parseWebSocketEvent([]byte(msg))
		assert.NoError(t, err)

		e, ok := raw.(*WebSocketOpEvent)
		assert.True(t, ok)
		assert.Equal(t, &WebSocketOpEvent{
			Success: nil,
			RetMsg:  nil,
			ConnId:  "civn4p1dcjmtvb69ome0-yrt1",
			ReqId:   nil,
			Op:      WsOpTypePong,
			Args:    []string{"1690884539181"},
		}, e)

		assert.NoError(t, e.IsValid())
	})

	t.Run("[private] PingEvent with req id", func(t *testing.T) {
		s := NewStream("", "")
		msg := `{"req_id":"78d36b57-a142-47b7-9143-5843df77d44d","op":"pong","args":["1690884539181"],"conn_id":"civn4p1dcjmtvb69ome0-yrt1"}`
		raw, err := s.parseWebSocketEvent([]byte(msg))
		assert.NoError(t, err)

		expReqId := "78d36b57-a142-47b7-9143-5843df77d44d"
		e, ok := raw.(*WebSocketOpEvent)
		assert.True(t, ok)
		assert.Equal(t, &WebSocketOpEvent{
			Success: nil,
			RetMsg:  nil,
			ConnId:  "civn4p1dcjmtvb69ome0-yrt1",
			ReqId:   &expReqId,
			Op:      WsOpTypePong,
			Args:    []string{"1690884539181"},
		}, e)

		assert.NoError(t, e.IsValid())
	})
}

func Test_WebSocketEventIsValid(t *testing.T) {
	t.Run("[public] valid op ping", func(t *testing.T) {
		expSucceeds := true
		expRetMsg := string(WsOpTypePong)
		expReqId := "b26704da-f5af-44c2-bdf7-935d6739e1a0"

		w := &WebSocketOpEvent{
			Success: &expSucceeds,
			RetMsg:  &expRetMsg,
			ReqId:   &expReqId,
			ConnId:  "test-conndid",
			Op:      WsOpTypePing,
			Args:    nil,
		}
		assert.NoError(t, w.IsValid())
	})

	t.Run("[private] valid op ping", func(t *testing.T) {
		w := &WebSocketOpEvent{
			Success: nil,
			RetMsg:  nil,
			ReqId:   nil,
			ConnId:  "test-conndid",
			Op:      WsOpTypePong,
			Args:    nil,
		}
		assert.NoError(t, w.IsValid())
	})

	t.Run("[public] un-Success", func(t *testing.T) {
		expSucceeds := false
		expRetMsg := string(WsOpTypePong)
		expReqId := "b26704da-f5af-44c2-bdf7-935d6739e1a0"

		w := &WebSocketOpEvent{
			Success: &expSucceeds,
			RetMsg:  &expRetMsg,
			ReqId:   &expReqId,
			ConnId:  "test-conndid",
			Op:      WsOpTypePing,
			Args:    nil,
		}
		assert.Error(t, fmt.Errorf("unexpeted response of pong: %+v", w), w.IsValid())
	})

	t.Run("[public] missing Success field", func(t *testing.T) {
		expRetMsg := string(WsOpTypePong)
		expReqId := "b26704da-f5af-44c2-bdf7-935d6739e1a0"

		w := &WebSocketOpEvent{
			RetMsg: &expRetMsg,
			ReqId:  &expReqId,
			ConnId: "test-conndid",
			Op:     WsOpTypePing,
			Args:   nil,
		}
		assert.Error(t, fmt.Errorf("unexpeted response of pong: %+v", w), w.IsValid())
	})

	t.Run("[public] invalid ret msg", func(t *testing.T) {
		expSucceeds := false
		expRetMsg := "PINGPONGPINGPONG"
		expReqId := "b26704da-f5af-44c2-bdf7-935d6739e1a0"

		w := &WebSocketOpEvent{
			Success: &expSucceeds,
			RetMsg:  &expRetMsg,
			ReqId:   &expReqId,
			ConnId:  "test-conndid",
			Op:      WsOpTypePing,
			Args:    nil,
		}
		assert.Error(t, fmt.Errorf("unexpeted response of pong: %+v", w), w.IsValid())
	})

	t.Run("[public] missing RetMsg field", func(t *testing.T) {
		expReqId := "b26704da-f5af-44c2-bdf7-935d6739e1a0"

		w := &WebSocketOpEvent{
			ReqId:  &expReqId,
			ConnId: "test-conndid",
			Op:     WsOpTypePing,
			Args:   nil,
		}
		assert.Error(t, fmt.Errorf("unexpeted response of pong: %+v", w), w.IsValid())
	})

	t.Run("unexpected op type", func(t *testing.T) {
		w := &WebSocketOpEvent{
			Op: WsOpType("unexpected"),
		}
		assert.Error(t, fmt.Errorf("unexpected op type: %+v", w), w.IsValid())
	})

	t.Run("[subscribe] valid", func(t *testing.T) {
		expSucceeds := true
		expRetMsg := ""
		w := &WebSocketOpEvent{
			Success: &expSucceeds,
			RetMsg:  &expRetMsg,
			ReqId:   nil,
			ConnId:  "test-conndid",
			Op:      WsOpTypeSubscribe,
			Args:    nil,
		}
		assert.NoError(t, w.IsValid())
	})

	t.Run("[subscribe] un-succeeds", func(t *testing.T) {
		expSucceeds := false
		expRetMsg := ""
		w := &WebSocketOpEvent{
			Success: &expSucceeds,
			RetMsg:  &expRetMsg,
			ReqId:   nil,
			ConnId:  "test-conndid",
			Op:      WsOpTypeSubscribe,
			Args:    nil,
		}
		assert.Error(t, fmt.Errorf("unexpected subscribe result: %+v", w), w.IsValid())
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

func Test_genTopicName(t *testing.T) {
	exp := "orderbook.50.BTCUSDT"
	assert.Equal(t, exp, genTopic(TopicTypeOrderBook, types.DepthLevel50, "BTCUSDT"))
}

func Test_getTopicName(t *testing.T) {
	exp := TopicTypeOrderBook
	assert.Equal(t, exp, getTopicType("orderbook.50.BTCUSDT"))
}

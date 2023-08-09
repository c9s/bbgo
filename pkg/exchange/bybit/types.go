package bybit

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type WsEvent struct {
	// "op" and "topic" are exclusive.
	*WebSocketOpEvent
	*WebSocketTopicEvent
}

func (w *WsEvent) IsOp() bool {
	return w.WebSocketOpEvent != nil && w.WebSocketTopicEvent == nil
}

func (w *WsEvent) IsTopic() bool {
	return w.WebSocketOpEvent == nil && w.WebSocketTopicEvent != nil
}

type WsOpType string

const (
	WsOpTypePing      WsOpType = "ping"
	WsOpTypePong      WsOpType = "pong"
	WsOpTypeAuth      WsOpType = "auth"
	WsOpTypeSubscribe WsOpType = "subscribe"
)

type WebsocketOp struct {
	Op   WsOpType `json:"op"`
	Args []string `json:"args"`
}

type WebSocketOpEvent struct {
	Success bool   `json:"success"`
	RetMsg  string `json:"ret_msg"`
	ReqId   string `json:"req_id,omitempty"`

	ConnId string   `json:"conn_id"`
	Op     WsOpType `json:"op"`
	Args   []string `json:"args"`
}

func (w *WebSocketOpEvent) IsValid() error {
	switch w.Op {
	case WsOpTypePing:
		// public event
		if !w.Success || WsOpType(w.RetMsg) != WsOpTypePong {
			return fmt.Errorf("unexpected response result: %+v", w)
		}
		return nil
	case WsOpTypePong:
		// private event, no success and ret_msg fields in response
		return nil
	case WsOpTypeAuth:
		if !w.Success || w.RetMsg != "" {
			return fmt.Errorf("unexpected response result: %+v", w)
		}
		return nil
	case WsOpTypeSubscribe:
		// in the public channel, you can get RetMsg = 'subscribe', but in the private channel, you cannot.
		// so, we only verify that success is true.
		if !w.Success {
			return fmt.Errorf("unexpected response result: %+v", w)
		}
		return nil
	default:
		return fmt.Errorf("unexpected op type: %+v", w)
	}
}

type TopicType string

const (
	TopicTypeOrderBook TopicType = "orderbook"
	TopicTypeWallet    TopicType = "wallet"
	TopicTypeOrder     TopicType = "order"
)

type DataType string

const (
	DataTypeSnapshot DataType = "snapshot"
	DataTypeDelta    DataType = "delta"
)

type WebSocketTopicEvent struct {
	Topic string   `json:"topic"`
	Type  DataType `json:"type"`
	// The timestamp (ms) that the system generates the data
	Ts   types.MillisecondTimestamp `json:"ts"`
	Data json.RawMessage            `json:"data"`
}

type BookEvent struct {
	// Symbol name
	Symbol string `json:"s"`
	// Bids. For snapshot stream, the element is sorted by price in descending order
	Bids types.PriceVolumeSlice `json:"b"`
	// Asks. For snapshot stream, the element is sorted by price in ascending order
	Asks types.PriceVolumeSlice `json:"a"`
	// Update ID. Is a sequence. Occasionally, you'll receive "u"=1, which is a snapshot data due to the restart of
	// the service. So please overwrite your local orderbook
	UpdateId fixedpoint.Value `json:"u"`
	// Cross sequence. You can use this field to compare different levels orderbook data, and for the smaller seq,
	// then it means the data is generated earlier.
	SequenceId fixedpoint.Value `json:"seq"`

	// internal use
	// Type can be one of snapshot or delta. Copied from WebSocketTopicEvent.Type
	Type DataType
}

func (e *BookEvent) OrderBook() (snapshot types.SliceOrderBook) {
	snapshot.Symbol = e.Symbol
	snapshot.Bids = e.Bids
	snapshot.Asks = e.Asks
	return snapshot
}

const topicSeparator = "."

func genTopic(in ...interface{}) string {
	out := make([]string, len(in))
	for k, v := range in {
		out[k] = fmt.Sprintf("%v", v)
	}
	return strings.Join(out, topicSeparator)
}

func getTopicType(topic string) TopicType {
	slice := strings.Split(topic, topicSeparator)
	if len(slice) == 0 {
		return ""
	}
	return TopicType(slice[0])
}

type OrderEvent struct {
	bybitapi.Order

	Category bybitapi.Category `json:"category"`
}

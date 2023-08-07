package bybit

import (
	"encoding/json"
	"fmt"
	"strings"

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
	WsOpTypeSubscribe WsOpType = "subscribe"
)

type WebsocketOp struct {
	Op   string   `json:"op"`
	Args []string `json:"args"`
}

type WebSocketOpEvent struct {
	Success *bool   `json:"success,omitempty"`
	RetMsg  *string `json:"ret_msg,omitempty"`
	ReqId   *string `json:"req_id,omitempty"`

	ConnId string   `json:"conn_id"`
	Op     WsOpType `json:"op"`
	Args   []string `json:"args"`
}

func (w *WebSocketOpEvent) IsValid() error {
	switch w.Op {
	case WsOpTypePing:
		// public event
		if (w.Success != nil && !*w.Success) ||
			(w.RetMsg != nil && WsOpType(*w.RetMsg) != WsOpTypePong) {
			return fmt.Errorf("unexpeted response of pong: %+v", w)
		}
		return nil
	case WsOpTypePong:
		// private event
		return nil
	case WsOpTypeSubscribe:
		if w.Success != nil && !*w.Success {
			return fmt.Errorf("unexpected subscribe result: %+v", w)
		}
		return nil
	default:
		return fmt.Errorf("unexpected op type: %+v", w)
	}
}

type TopicType string

const (
	TopicTypeOrderBook TopicType = "orderbook"
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

// PriceVolumeSlice represents a slice of price and value.
//
// index 0 is Bid/Ask price.
// index 1 is Bid/Ask size. The *delta data* has size=0, which means that all quotations for this price have been filled or cancelled
type PriceVolumeSlice [2]fixedpoint.Value

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

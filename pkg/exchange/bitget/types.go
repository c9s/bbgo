package bitget

import (
	"encoding/json"
	"fmt"

	"github.com/c9s/bbgo/pkg/types"
)

type InstType string

const (
	instSp InstType = "sp"
)

type ChannelType string

const (
	// ChannelOrderBook snapshot and update might return less than 200 bids/asks as per symbol's orderbook various from
	// each other; The number of bids/asks is not a fixed value and may vary in the future
	ChannelOrderBook ChannelType = "books"
	// ChannelOrderBook5 top 5 order book of "books" that begins from bid1/ask1
	ChannelOrderBook5 ChannelType = "books5"
	// ChannelOrderBook15 top 15 order book of "books" that begins from bid1/ask1
	ChannelOrderBook15 ChannelType = "books15"
)

type WsArg struct {
	InstType InstType    `json:"instType"`
	Channel  ChannelType `json:"channel"`
	// InstId Instrument ID. e.q. BTCUSDT, ETHUSDT
	InstId string `json:"instId"`
}

type WsEventType string

const (
	WsEventSubscribe   WsEventType = "subscribe"
	WsEventUnsubscribe WsEventType = "unsubscribe"
	WsEventError       WsEventType = "error"
)

type WsOp struct {
	Op   WsEventType `json:"op"`
	Args []WsArg     `json:"args"`
}

// WsEvent is the lowest level of event type. We use this struct to convert the received data, so that we will know
// whether the event belongs to `op` or `data`.
type WsEvent struct {
	// for comment event
	Arg WsArg `json:"arg"`

	// for op event
	Event WsEventType `json:"event"`
	Code  int         `json:"code"`
	Msg   string      `json:"msg"`
	Op    string      `json:"op"`

	// for data event
	Action ActionType      `json:"action"`
	Data   json.RawMessage `json:"data"`
}

// IsOp represents the data event will be empty
func (w *WsEvent) IsOp() bool {
	return w.Action == "" && len(w.Data) == 0
}

func (w *WsEvent) IsValid() error {
	switch w.Event {
	case WsEventError:
		return fmt.Errorf("websocket request error, op: %s, code: %d, msg: %s", w.Op, w.Code, w.Msg)

	case WsEventSubscribe, WsEventUnsubscribe:
		// Actually, this code is unnecessary because the events are either `Subscribe` or `Unsubscribe`, But to avoid bugs
		// in the exchange, we still check.
		if w.Code != 0 || len(w.Msg) != 0 {
			return fmt.Errorf("unexpected websocket %s event, code: %d, msg: %s", w.Event, w.Code, w.Msg)
		}
		return nil

	default:
		return fmt.Errorf("unexpected event type: %+v", w)
	}
}

type ActionType string

const (
	ActionTypeSnapshot ActionType = "snapshot"
	ActionTypeUpdate   ActionType = "update"
)

//	{
//	   "asks":[
//	      [
//	         "28350.78",
//	         "0.2082"
//	      ],
//	   ],
//	   "bids":[
//	      [
//	         "28350.70",
//	         "0.5585"
//	      ],
//	   ],
//	   "checksum":0,
//	   "ts":"1697593934630"
//	}
type BookEvent struct {
	Events []struct {
		// Order book on sell side, ascending order
		Asks types.PriceVolumeSlice `json:"asks"`
		// Order book on buy side, descending order
		Bids     types.PriceVolumeSlice     `json:"bids"`
		Ts       types.MillisecondTimestamp `json:"ts"`
		Checksum int                        `json:"checksum"`
	}

	// internal use
	Type   ActionType
	InstId string
}

func (e *BookEvent) ToGlobalOrderBooks() []types.SliceOrderBook {
	books := make([]types.SliceOrderBook, len(e.Events))
	for i, event := range e.Events {
		books[i] = types.SliceOrderBook{
			Symbol: e.InstId,
			Bids:   event.Bids,
			Asks:   event.Asks,
			Time:   event.Ts.Time(),
		}
	}

	return books
}

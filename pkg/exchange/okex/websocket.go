package okex

import (
	"encoding/json"

	"github.com/c9s/bbgo/pkg/types"
)

type WsEvent struct {
	// "op" and "PushData" are exclusive.
	*WebSocketOpEvent
	*WebSocketPushDataEvent
}

func (w *WsEvent) IsOp() bool {
	return w.WebSocketOpEvent != nil && w.WebSocketPushDataEvent == nil
}

func (w *WsEvent) IsPushDataEvent() bool {
	return w.WebSocketOpEvent == nil && w.WebSocketPushDataEvent != nil
}

type WsOpType string

const (
	WsOpTypeLogin WsOpType = "login"
	// subscribe and unsubscribe could be public or private, ex. private for chennel: orders
	WsOpTypeSubscribe         WsOpType = "subscribe"
	WsOpTypeUnsubscribe       WsOpType = "unsubscribe"
	WsOpTypeOrder             WsOpType = "order"
	WsOpTypeBatchOrders       WsOpType = "batch-orders"
	WsOpTypeCancelOrder       WsOpType = "cancel-order"
	WsOpTypeBatchCancelOrders WsOpType = "batch-cancel-orders"
	WsOpTypeAmendOrder        WsOpType = "amend-order"
	WsOpTypeBatchAmendOrders  WsOpType = "batch-amend-orders"
	WsOpTypeMassCancel        WsOpType = "mass-cancel"
	// below type exist only in response
	WsOpTypeError WsOpType = "error"
)

// Websocket Op
type WebsocketOp struct {
	// id only applicable to private op, ex, order, batch-orders
	Id   string                  `json:"id,omitempty"`
	Op   WsOpType                `json:"op"`
	Args []WebsocketSubscription `json:"args"`
}

// Websocket Op event
type WebSocketOpEvent struct {
	// id only applicable to private op, ex, order, batch-orders
	Id   string                  `json:"id,omitempty"`
	Op   WsOpType                `json:"op"`
	Args []WebsocketSubscription `json:"args,omitempty"`
	// Below is Websocket Response field
	Event   WsOpType                `json:"event,omitempty"`
	Code    string                  `json:"code,omitempty"`
	Message string                  `json:"msg,omitempty"`
	Arg     []WebsocketSubscription `json:"arg,omitempty"`
}

// Websocket Response event for private channel
type WebSocketPrivateEvent struct {
	Id      string                     `json:"id"`
	Op      WsOpType                   `json:"op"`
	Data    json.RawMessage            `json:"data"`
	Code    string                     `json:"code"`
	Message string                     `json:"msg"`
	InTime  types.MillisecondTimestamp `json:"inTime,omitempty"`
	OutTime types.MillisecondTimestamp `json:"outTime,omitempty"`
}

// Websocket Push data event
type WebSocketPushDataEvent struct {
	Arg  WebsocketSubscription `json:"arg"`
	Data json.RawMessage       `json:"data"`
	// action: snapshot, update, only applicable to : channel (books)
	Action *string `json:"action,omitempty"`
}

type WebSocketChannelType string

const (
	// below channel need authenticated
	WsChannelTypeAccount            WebSocketChannelType = "account"
	WsChannelTypePositions          WebSocketChannelType = "positions"
	WsChannelTypeBalanceAndPosition WebSocketChannelType = "balance_and_position"
	WsChannelTypeLiquidationWarning WebSocketChannelType = "liquidation-warning"
	WsChannelTypeAccountGreeks      WebSocketChannelType = "account-greeks"
	WsChannelTypeOrders             WebSocketChannelType = "orders"
	// below channel no need authenticated
	WsChannelTypeTickers      WebSocketChannelType = "tickers"
	WsChannelTypeTrades       WebSocketChannelType = "trades"
	WsChannelTypeTradesAll    WebSocketChannelType = "trades-all"
	WsChannelTypeOptionTrades WebSocketChannelType = "option-trades"
	WsChannelTypeBooks        WebSocketChannelType = "books"
	WsChannelTypeBooks5       WebSocketChannelType = "books5"
	WsChannelTypeBooks50L2Tbt WebSocketChannelType = "books50-l2-tbt"
	WsChannelTypeBooksL2Tbt   WebSocketChannelType = "books-l2-tbt"
)

func (w *WebsocketSubscription) NeedAuthenticated() bool {
	return w.Channel != string(WsChannelTypeTickers) && w.Channel != string(WsChannelTypeTrades) &&
		w.Channel != string(WsChannelTypeTradesAll) && w.Channel != string(WsChannelTypeOptionTrades)
}

func (w *WebSocketOpEvent) IsAuthenticated() bool {
	return w.Op == WsOpTypeLogin && w.Code == "0"
}

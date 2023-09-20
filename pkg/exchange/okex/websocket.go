package okex

import (
	"encoding/json"
)

type WsEvent struct {
	// "op" and "topic" (push data) are exclusive.
	*WebSocketEvent
	*WebSocketTopicEvent
}

func (w *WsEvent) IsOp() bool {
	return w.WebSocketEvent != nil && w.WebSocketTopicEvent == nil
}

func (w *WsEvent) IsTopicEvent() bool {
	return w.WebSocketEvent == nil && w.WebSocketTopicEvent != nil
}

type WsOpType string

const (
	WsOpTypeLogin             WsOpType = "login"
	WsOpTypeSubscribe         WsOpType = "subscribe"
	WsOpTypeUnsubscribe       WsOpType = "unsubscribe"
	WsOpTypeOrder             WsOpType = "order"
	WsOpTypeBatchOrders       WsOpType = "batch-orders"
	WsOpTypeCancelOrder       WsOpType = "cancel-order"
	WsOpTypeBatchCancelOrders WsOpType = "batch-cancel-orders"
	WsOpTypeAmendOrder        WsOpType = "amend-order"
	WsOpTypeBatchAmendOrders  WsOpType = "batch-amend-orders"
	WsOpTypeMassCancel        WsOpType = "mass-cancel"
	// below type exist in response
	WsOpTypeError WsOpType = "error"
)

type WebsocketOp struct {
	Op   WsOpType                `json:"op"`
	Args []WebsocketSubscription `json:"args"`
}

type WebSocketEvent struct {
	Id      string                  `json:"id,omitempty"`
	Event   string                  `json:"event"`
	Code    string                  `json:"code,omitempty"`
	Message string                  `json:"msg,omitempty"`
	Arg     []WebsocketSubscription `json:"arg,omitempty"`
	Data    json.RawMessage         `json:"data,omitempty"`
}

type WebSocketTopicEvent struct {
	Arg []WebsocketSubscription `json:"arg"`
	// action: snapshot, update
	Action *string         `json:"action,omitempty"`
	Data   json.RawMessage `json:"data"`
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
)

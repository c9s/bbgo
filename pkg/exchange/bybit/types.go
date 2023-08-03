package bybit

import (
	"fmt"
)

type WsOpType string

const (
	WsOpTypePing WsOpType = "ping"
	WsOpTypePong WsOpType = "pong"
)

type WebSocketEvent struct {
	Success *bool   `json:"success,omitempty"`
	RetMsg  *string `json:"ret_msg,omitempty"`
	ReqId   *string `json:"req_id,omitempty"`

	ConnId string   `json:"conn_id"`
	Op     WsOpType `json:"op"`
	Args   []string `json:"args"`
}

func (w *WebSocketEvent) IsValid() error {
	switch w.Op {
	case WsOpTypePing:
		// public event
		if (w.Success != nil && !*w.Success) ||
			(w.RetMsg != nil && WsOpType(*w.RetMsg) != WsOpTypePong) {
			return fmt.Errorf("unexpeted response of pong: %#v", w)
		}
		return nil
	case WsOpTypePong:
		// private event
		return nil
	default:
		return fmt.Errorf("unexpected op type: %#v", w)
	}
}

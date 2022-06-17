package max

import (
	"github.com/pkg/errors"
)

var WebSocketURL = "wss://max-stream.maicoin.com/ws"

var ErrMessageTypeNotSupported = errors.New("message type currently not supported")

type SubscribeOptions struct {
	Depth      int    `json:"depth,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

// Subscription is used for presenting the subscription metadata.
// This is used for sending subscribe and unsubscribe requests
type Subscription struct {
	Channel    string `json:"channel"`
	Market     string `json:"market"`
	Depth      int    `json:"depth,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

type WebsocketCommand struct {
	// Action is used for specify the action of the websocket session.
	// Valid values are "subscribe", "unsubscribe" and "auth"
	Action        string         `json:"action"`
	Subscriptions []Subscription `json:"subscriptions,omitempty"`
}

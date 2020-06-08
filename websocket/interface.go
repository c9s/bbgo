package websocket

import (
	"context"
	"time"
)

//go:generate mockery -name=Client

type Client interface {
	SetWriteTimeout(time.Duration)
	SetReadTimeout(time.Duration)
	OnConnect(func(c Client))
	OnDisconnect(func(c Client))
	Connect(context.Context) error
	Reconnect()
	Close() error
	IsConnected() bool
	WriteJSON(interface{}) error
	Messages() <-chan Message
}

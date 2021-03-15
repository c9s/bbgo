package types

import (
	"context"
)

type Stream interface {
	StandardStreamEventHub

	Subscribe(channel Channel, symbol string, options SubscribeOptions)
	SetPublicOnly()
	Connect(ctx context.Context) error
	Close() error
}

type Channel string

var BookChannel = Channel("book")

var KLineChannel = Channel("kline")

//go:generate callbackgen -type StandardStream -interface
type StandardStream struct {
	Subscriptions []Subscription

	connectCallbacks []func()

	disconnectCallbacks []func()

	// private trade update callbacks
	tradeUpdateCallbacks []func(trade Trade)

	// private order update callbacks
	orderUpdateCallbacks []func(order Order)

	// balance snapshot callbacks
	balanceSnapshotCallbacks []func(balances BalanceMap)

	balanceUpdateCallbacks []func(balances BalanceMap)

	kLineClosedCallbacks []func(kline KLine)

	kLineCallbacks []func(kline KLine)

	bookUpdateCallbacks []func(book OrderBook)

	bookSnapshotCallbacks []func(book OrderBook)
}

func (stream *StandardStream) Subscribe(channel Channel, symbol string, options SubscribeOptions) {
	stream.Subscriptions = append(stream.Subscriptions, Subscription{
		Channel: channel,
		Symbol:  symbol,
		Options: options,
	})
}

// SubscribeOptions provides the standard stream options
type SubscribeOptions struct {
	Interval string `json:"interval,omitempty"`
	Depth    string `json:"depth,omitempty"`
}

func (o SubscribeOptions) String() string {
	if len(o.Interval) > 0 {
		return o.Interval
	}

	return o.Depth
}

type Subscription struct {
	Symbol  string           `json:"symbol"`
	Channel Channel          `json:"channel"`
	Options SubscribeOptions `json:"options"`
}

package types

import (
	"context"
)

type Stream interface {
	StandardStreamEventHub

	Subscribe(channel Channel, symbol string, options SubscribeOptions)
	Connect(ctx context.Context) error
	Close() error
}

type Channel string

var BookChannel = Channel("book")

var KLineChannel = Channel("kline")

//go:generate callbackgen -type StandardStream -interface
type StandardStream struct {
	Subscriptions []Subscription

	// private trade callbacks
	tradeCallbacks []func(trade *Trade)

	// balance snapshot callbacks
	balanceSnapshotCallbacks []func(balances map[string]Balance)

	balanceUpdateCallbacks []func(balances map[string]Balance)

	kLineClosedCallbacks []func(kline KLine)

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
	Interval string
	Depth    string
}

func (o SubscribeOptions) String() string {
	if len(o.Interval) > 0 {
		return o.Interval
	}

	return o.Depth
}

type Subscription struct {
	Symbol  string
	Channel Channel
	Options SubscribeOptions
}

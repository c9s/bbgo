package types

import (
	"fmt"
	"strings"
)

//go:generate callbackgen -type StandardPrivateStream -interface
type StandardPrivateStream struct {
	Subscriptions []Subscription

	tradeCallbacks   []func(trade *Trade)
	balanceSnapshotCallbacks []func(balanceSnapshot map[string]Balance)
	kLineClosedCallbacks       []func(kline *KLine)
}

func (stream *StandardPrivateStream) Subscribe(channel string, symbol string, options SubscribeOptions) {
	stream.Subscriptions = append(stream.Subscriptions, Subscription{
		Channel: channel,
		Symbol:  symbol,
		Options: options,
	})
}


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
	Channel string
	Options SubscribeOptions
}

func (s *Subscription) String() string {
	// binance uses lower case symbol name
	return fmt.Sprintf("%s@%s_%s", strings.ToLower(s.Symbol), s.Channel, s.Options.String())
}


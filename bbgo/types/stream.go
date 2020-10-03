package types

import (
	"context"
	"fmt"
	"strings"
)

type PrivateStream interface {
	StandardStreamEventHub

	Subscribe(channel string, symbol string, options SubscribeOptions)
	Connect(ctx context.Context) error
	Close() error
}

//go:generate callbackgen -type StandardStream -interface
type StandardStream struct {
	Subscriptions []Subscription

	// private trade callbacks
	tradeCallbacks   []func(trade *Trade)

	// balance snapshot callbacks
	balanceSnapshotCallbacks []func(balances map[string]Balance)

	balanceUpdateCallbacks []func(balances map[string]Balance)

	kLineClosedCallbacks       []func(kline KLine)

	bookUpdateCallbacks []func(book OrderBook)

	bookSnapshotCallbacks []func(book OrderBook)
}

func (stream *StandardStream) Subscribe(channel string, symbol string, options SubscribeOptions) {
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
	// binance uses lower case symbol name,
	// for kline, it's "<symbol>@kline_<interval>"
	// for depth, it's "<symbol>@depth OR <symbol>@depth@100ms"
	switch s.Channel {
	case "kline":
		return fmt.Sprintf("%s@%s_%s", strings.ToLower(s.Symbol), s.Channel, s.Options.String())
	case "depth", "book":
		return fmt.Sprintf("%s@%s", strings.ToLower(s.Symbol), s.Channel)
	default:
		return fmt.Sprintf("%s@%s", strings.ToLower(s.Symbol), s.Channel)
	}
}


package bbgo

import "github.com/c9s/bbgo/pkg/types"

// ExchangeSession presents the exchange connection session
// It also maintains and collects the data returned from the stream.
type ExchangeSession struct {
	// Exchange session name
	Name string

	// The exchange account states
	Account *Account

	// Stream is the connection stream of the exchange
	Stream types.Stream

	Subscriptions map[types.Subscription]types.Subscription

	Exchange types.Exchange

	// Markets defines market configuration of a symbol
	Markets map[string]types.Market

	LastPrices map[string]float64

	// Trades collects the executed trades from the exchange
	// map: symbol -> []trade
	Trades map[string][]types.Trade

	MarketDataStores map[string]*MarketDataStore
}

func NewExchangeSession(name string, exchange types.Exchange) *ExchangeSession {
	return &ExchangeSession{
		Name:          name,
		Exchange:      exchange,
		Subscriptions: make(map[types.Subscription]types.Subscription),
		Markets:       make(map[string]types.Market),
		Trades:        make(map[string][]types.Trade),
		LastPrices:    make(map[string]float64),
		MarketDataStores: make(map[string]*MarketDataStore),
	}
}

// Subscribe save the subscription info, later it will be assigned to the stream
func (session *ExchangeSession) Subscribe(channel types.Channel, symbol string, options types.SubscribeOptions) *ExchangeSession {
	sub := types.Subscription{
		Channel: channel,
		Symbol:  symbol,
		Options: options,
	}

	session.Subscriptions[sub] = sub
	return session
}

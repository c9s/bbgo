package bbgo

import (
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/store"
	"github.com/c9s/bbgo/pkg/types"
)

type IntervalWindow struct {
	// The interval of kline
	Interval types.Interval

	// The windows size of EWMA and SMA
	Window int
}

type StandardIndicatorSet struct {
	Symbol string
	// Standard indicators
	// interval -> window
	SMA  map[IntervalWindow]*indicator.SMA
	EWMA map[IntervalWindow]*indicator.EWMA
}

func NewStandardIndicatorSet(symbol string) *StandardIndicatorSet {
	set := &StandardIndicatorSet{
		Symbol: symbol,
		SMA:    make(map[IntervalWindow]*indicator.SMA),
		EWMA:   make(map[IntervalWindow]*indicator.EWMA),
	}

	// let us pre-defined commonly used intervals
	for interval := range types.SupportedIntervals {
		for _, window := range []int{7, 25, 99} {
			set.SMA[IntervalWindow{interval, window}] = &indicator.SMA{
				Interval: interval,
				Window:   window,
			}
			set.EWMA[IntervalWindow{interval, window}] = &indicator.EWMA{
				Interval: interval,
				Window:   window,
			}
		}
	}

	return set
}

func (set *StandardIndicatorSet) BindMarketDataStore(store *store.MarketDataStore) {
	for _, inc := range set.SMA {
		inc.BindMarketDataStore(store)
	}

	for _, inc := range set.EWMA {
		inc.BindMarketDataStore(store)
	}
}

// ExchangeSession presents the exchange connection session
// It also maintains and collects the data returned from the stream.
type ExchangeSession struct {
	// Exchange session name
	Name string

	// The exchange account states
	Account *types.Account

	// Stream is the connection stream of the exchange
	Stream types.Stream

	Subscriptions map[types.Subscription]types.Subscription

	Exchange types.Exchange

	// markets defines market configuration of a symbol
	markets map[string]types.Market

	lastPrices map[string]float64

	// Trades collects the executed trades from the exchange
	// map: symbol -> []trade
	Trades map[string][]types.Trade

	// marketDataStores contains the market data store of each market
	marketDataStores map[string]*store.MarketDataStore

	// standard indicators of each market
	standardIndicatorSets map[string]*StandardIndicatorSet

	tradeReporter *TradeReporter
}

func NewExchangeSession(name string, exchange types.Exchange) *ExchangeSession {
	return &ExchangeSession{
		Name:             name,
		Exchange:         exchange,
		Stream:           exchange.NewStream(),
		Account:          &types.Account{},
		Subscriptions:    make(map[types.Subscription]types.Subscription),
		markets:          make(map[string]types.Market),
		Trades:           make(map[string][]types.Trade),
		lastPrices:       make(map[string]float64),
		marketDataStores: make(map[string]*store.MarketDataStore),
	}
}

func (session *ExchangeSession) StandardIndicatorSet(symbol string) (*StandardIndicatorSet, bool) {
	set, ok := session.standardIndicatorSets[symbol]
	return set, ok
}

// MarketDataStore returns the market data store of a symbol
func (session *ExchangeSession) MarketDataStore(symbol string) (s *store.MarketDataStore, ok bool) {
	s, ok = session.marketDataStores[symbol]
	return s, ok
}

func (session *ExchangeSession) LastPrice(symbol string) (price float64, ok bool) {
	price, ok = session.lastPrices[symbol]
	return price, ok
}

func (session *ExchangeSession) Market(symbol string) (market types.Market, ok bool) {
	market, ok = session.markets[symbol]
	return market, ok
}

func (session *ExchangeSession) ReportTrade(notifier Notifier) *TradeReporter {
	session.tradeReporter = NewTradeReporter(notifier)
	return session.tradeReporter
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

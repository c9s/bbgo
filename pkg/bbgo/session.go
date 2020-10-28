package bbgo

import (
	"github.com/c9s/bbgo/pkg/indicator"
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

	store *MarketDataStore
}

func NewStandardIndicatorSet(symbol string, store *MarketDataStore) *StandardIndicatorSet {
	set := &StandardIndicatorSet{
		Symbol: symbol,
		SMA:    make(map[IntervalWindow]*indicator.SMA),
		EWMA:   make(map[IntervalWindow]*indicator.EWMA),
		store:  store,
	}

	// let us pre-defined commonly used intervals
	for interval := range types.SupportedIntervals {
		for _, window := range []int{7, 25, 99} {
			iw := IntervalWindow{interval, window}
			set.SMA[iw] = &indicator.SMA{Interval: interval, Window: window}
			set.SMA[iw].Bind(store)

			set.EWMA[iw] = &indicator.EWMA{Interval: interval, Window: window}
			set.EWMA[iw].Bind(store)
		}
	}

	return set
}

func (set *StandardIndicatorSet) GetSMA(iw IntervalWindow) *indicator.SMA {
	inc, ok := set.SMA[iw]
	if !ok {
		inc := &indicator.SMA{Interval: iw.Interval, Window: iw.Window}
		inc.Bind(set.store)
		set.SMA[iw] = inc
	}

	return inc
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
	marketDataStores map[string]*MarketDataStore

	// standard indicators of each market
	standardIndicatorSets map[string]*StandardIndicatorSet

	loadedSymbols map[string]struct{}
}

func NewExchangeSession(name string, exchange types.Exchange) *ExchangeSession {
	return &ExchangeSession{
		Name:          name,
		Exchange:      exchange,
		Stream:        exchange.NewStream(),
		Subscriptions: make(map[types.Subscription]types.Subscription),
		Account:       &types.Account{},
		Trades:        make(map[string][]types.Trade),

		markets:               make(map[string]types.Market),
		lastPrices:            make(map[string]float64),
		marketDataStores:      make(map[string]*MarketDataStore),
		standardIndicatorSets: make(map[string]*StandardIndicatorSet),

		loadedSymbols: make(map[string]struct{}),
	}
}

func (session *ExchangeSession) StandardIndicatorSet(symbol string) (*StandardIndicatorSet, bool) {
	set, ok := session.standardIndicatorSets[symbol]
	return set, ok
}

// MarketDataStore returns the market data store of a symbol
func (session *ExchangeSession) MarketDataStore(symbol string) (s *MarketDataStore, ok bool) {
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

// Subscribe save the subscription info, later it will be assigned to the stream
func (session *ExchangeSession) Subscribe(channel types.Channel, symbol string, options types.SubscribeOptions) *ExchangeSession {
	sub := types.Subscription{
		Channel: channel,
		Symbol:  symbol,
		Options: options,
	}

	// add to the loaded symbol table
	session.loadedSymbols[symbol] = struct{}{}
	session.Subscriptions[sub] = sub
	return session
}

package indicatorv2

import "github.com/c9s/bbgo/pkg/types"

const MaxNumOfTrades = 5_000

//go:generate callbackgen -type TradeStream
type TradeStream struct {
	updateCallbacks []func(trade types.Trade)
	trades          []types.Trade
}

func (s *TradeStream) Length() int {
	return len(s.trades)
}

func (s *TradeStream) Last(i int) *types.Trade {
	l := len(s.trades)
	if i < 0 || l-1-i < 0 {
		return nil
	}

	return &s.trades[l-1-i]
}

func (s *TradeStream) Tail(n int) []types.Trade {
	l := len(s.trades)
	if n >= l {
		return s.trades
	}

	return s.trades[l-n : l]
}

// AddSubscriber adds the subscriber function and push historical data to the subscriber
func (s *TradeStream) AddSubscriber(f func(trade types.Trade)) {
	s.OnUpdate(f)

	if len(s.trades) == 0 {
		return
	}

	// push historical trades to the subscriber
	for _, trade := range s.trades {
		f(trade)
	}
}

func (s *TradeStream) BackFill(trades []types.Trade) {
	for _, trade := range trades {
		s.trades = append(s.trades, trade)
		s.EmitUpdate(trade)
	}
}

// Trades creates a Trade stream that pushes the trades to the subscribers
func Trades(source types.Stream, symbol string) *TradeStream {
	s := &TradeStream{}

	source.OnMarketTrade(types.TradeWith(symbol, func(trade types.Trade) {
		s.trades = append(s.trades, trade)
		s.EmitUpdate(trade)

		s.trades = types.ShrinkSlice(s.trades, MaxNumOfTrades, MaxNumOfTrades/5)
	}))

	return s
}

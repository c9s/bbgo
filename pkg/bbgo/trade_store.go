package bbgo

import (
	"sync"

	"github.com/c9s/bbgo/pkg/types"
)

type TradeStore struct {
	// any created trades for tracking trades
	mu     sync.Mutex
	trades map[int64]types.Trade

	Symbol          string
	RemoveCancelled bool
	RemoveFilled    bool
	AddOrderUpdate  bool
}

func NewTradeStore(symbol string) *TradeStore {
	return &TradeStore{
		Symbol: symbol,
		trades: make(map[int64]types.Trade),
	}
}

func (s *TradeStore) Num() (num int) {
	s.mu.Lock()
	num = len(s.trades)
	s.mu.Unlock()
	return num
}

func (s *TradeStore) Trades() (trades []types.Trade) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, o := range s.trades {
		trades = append(trades, o)
	}

	return trades
}

func (s *TradeStore) Exists(oID int64) (ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok = s.trades[oID]
	return ok
}

func (s *TradeStore) Clear() {
	s.mu.Lock()
	s.trades = make(map[int64]types.Trade)
	s.mu.Unlock()
}

func (s *TradeStore) GetAndClear() (trades []types.Trade) {
	s.mu.Lock()
	for _, o := range s.trades {
		trades = append(trades, o)
	}
	s.trades = make(map[int64]types.Trade)
	s.mu.Unlock()

	return trades
}

func (s *TradeStore) Add(trades ...types.Trade) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, trade := range trades {
		s.trades[trade.ID] = trade
	}
}

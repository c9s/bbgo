package bbgo

import (
	"sync"

	"github.com/c9s/bbgo/pkg/types"
)

type TradeStore struct {
	// any created trades for tracking trades
	sync.Mutex

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
	s.Lock()
	num = len(s.trades)
	s.Unlock()
	return num
}

func (s *TradeStore) Trades() (trades []types.Trade) {
	s.Lock()
	defer s.Unlock()

	for _, o := range s.trades {
		trades = append(trades, o)
	}

	return trades
}

func (s *TradeStore) Exists(oID int64) (ok bool) {
	s.Lock()
	defer s.Unlock()

	_, ok = s.trades[oID]
	return ok
}

func (s *TradeStore) Clear() {
	s.Lock()
	s.trades = make(map[int64]types.Trade)
	s.Unlock()
}

type TradeFilter func(trade types.Trade) bool

func (s *TradeStore) Filter(filter TradeFilter) {
	s.Lock()
	var trades = make(map[int64]types.Trade)
	for _, trade := range s.trades {
		if filter(trade) {
			trades[trade.ID] = trade
		}
	}
	s.trades = trades
	s.Unlock()
}

func (s *TradeStore) GetAndClear() (trades []types.Trade) {
	s.Lock()
	for _, o := range s.trades {
		trades = append(trades, o)
	}
	s.trades = make(map[int64]types.Trade)
	s.Unlock()

	return trades
}

func (s *TradeStore) Add(trades ...types.Trade) {
	s.Lock()
	defer s.Unlock()

	for _, trade := range trades {
		s.trades[trade.ID] = trade
	}
}

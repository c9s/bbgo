package core

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

const TradeExpiryTime = 3 * time.Hour
const CoolTradePeriod = 1 * time.Hour
const MaximumTradeStoreSize = 1_000

type TradeStore struct {
	// any created trades for tracking trades
	sync.Mutex

	EnablePrune bool

	trades        map[uint64]types.Trade
	lastTradeTime time.Time
}

func NewTradeStore() *TradeStore {
	return &TradeStore{
		trades: make(map[uint64]types.Trade),
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

func (s *TradeStore) Exists(oID uint64) (ok bool) {
	s.Lock()
	defer s.Unlock()

	_, ok = s.trades[oID]
	return ok
}

func (s *TradeStore) Clear() {
	s.Lock()
	s.trades = make(map[uint64]types.Trade)
	s.Unlock()
}

type TradeFilter func(trade types.Trade) bool

// Filter filters the trades by a given TradeFilter function
func (s *TradeStore) Filter(filter TradeFilter) {
	s.Lock()
	var trades = make(map[uint64]types.Trade)
	for _, trade := range s.trades {
		if !filter(trade) {
			trades[trade.ID] = trade
		}
	}
	s.trades = trades
	s.Unlock()
}

// GetOrderTrades finds the trades match order id matches to the given order
func (s *TradeStore) GetOrderTrades(o types.Order) (trades []types.Trade) {
	s.Lock()
	for _, t := range s.trades {
		if t.OrderID == o.OrderID {
			trades = append(trades, t)
		}
	}
	s.Unlock()
	return trades
}

func (s *TradeStore) GetAndClear() (trades []types.Trade) {
	s.Lock()
	for _, t := range s.trades {
		trades = append(trades, t)
	}
	s.trades = make(map[uint64]types.Trade)
	s.Unlock()

	return trades
}

func (s *TradeStore) Add(trades ...types.Trade) {
	s.Lock()
	defer s.Unlock()

	for _, trade := range trades {
		s.trades[trade.ID] = trade
		s.touchLastTradeTime(trade)
	}
}

func (s *TradeStore) touchLastTradeTime(trade types.Trade) {
	if trade.Time.Time().After(s.lastTradeTime) {
		s.lastTradeTime = trade.Time.Time()
	}
}

// Prune prunes trades that are older than the expiry time
// see TradeExpiryTime (3 hours)
func (s *TradeStore) Prune(curTime time.Time) {
	s.Lock()
	defer s.Unlock()

	var trades = make(map[uint64]types.Trade)
	var cutOffTime = curTime.Add(-TradeExpiryTime)

	log.Infof("pruning expired trades, cutoff time = %s", cutOffTime.String())
	for _, trade := range s.trades {
		if trade.Time.Before(cutOffTime) {
			continue
		}

		trades[trade.ID] = trade
	}

	s.trades = trades

	log.Infof("trade pruning done, size: %d", len(trades))
}

func (s *TradeStore) isCoolTrade(trade types.Trade) bool {
	// if the duration between the current trade and the last trade is over 1 hour, we call it "cool trade"
	return !s.lastTradeTime.IsZero() && time.Time(trade.Time).Sub(s.lastTradeTime) > CoolTradePeriod
}

func (s *TradeStore) exceededMaximumTradeStoreSize() bool {
	return len(s.trades) > MaximumTradeStoreSize
}

func (s *TradeStore) BindStream(stream types.Stream) {
	stream.OnTradeUpdate(func(trade types.Trade) {
		s.Add(trade)
	})

	if s.EnablePrune {
		stream.OnTradeUpdate(func(trade types.Trade) {
			if s.isCoolTrade(trade) || s.exceededMaximumTradeStoreSize() {
				s.Prune(time.Time(trade.Time))
			}
		})
	}
}

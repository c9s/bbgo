package xgap

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/max"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "xgap"

const stateKey = "state-v1"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

func (s *Strategy) ID() string {
	return ID
}

type State struct {
	AccumulatedFeeStartedAt time.Time                   `json:"accumulatedFeeStartedAt,omitempty"`
	AccumulatedFees         map[string]fixedpoint.Value `json:"accumulatedFees,omitempty"`
	AccumulatedVolume       fixedpoint.Value            `json:"accumulatedVolume,omitempty"`
}

func (s *State) IsOver24Hours() bool {
	return time.Now().Sub(s.AccumulatedFeeStartedAt) >= 24*time.Hour
}

func (s *State) Reset() {
	t := time.Now()
	dateTime := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())

	log.Infof("resetting accumulated started time to: %s", dateTime)

	s.AccumulatedFeeStartedAt = dateTime
	s.AccumulatedFees = make(map[string]fixedpoint.Value)
	s.AccumulatedVolume = 0
}

type Strategy struct {
	*bbgo.Graceful
	*bbgo.Notifiability
	*bbgo.Persistence

	Symbol          string           `json:"symbol"`
	SourceExchange  string           `json:"sourceExchange"`
	TradingExchange string           `json:"tradingExchange"`
	Quantity        fixedpoint.Value `json:"quantity"`

	DailyFeeBudgets map[string]fixedpoint.Value `json:"dailyFeeBudgets,omitempty"`
	DailyMaxVolume  fixedpoint.Value            `json:"dailyMaxVolume,omitempty"`
	UpdateInterval  types.Duration              `json:"updateInterval"`
	SimulateVolume  bool                        `json:"simulateVolume"`

	sourceSession, tradingSession *bbgo.ExchangeSession
	sourceMarket, tradingMarket   types.Market

	state *State

	mu                                sync.Mutex
	lastSourceKLine, lastTradingKLine types.KLine
	sourceBook, tradingBook           *types.StreamOrderBook
	groupID                           uint32

	stopC chan struct{}
}

func (s *Strategy) isBudgetAllowed() bool {
	if s.DailyFeeBudgets == nil {
		return true
	}

	if s.state.AccumulatedFees == nil {
		return true
	}

	for asset, budget := range s.DailyFeeBudgets {
		if fee, ok := s.state.AccumulatedFees[asset]; ok {
			if fee >= budget {
				log.Warnf("accumulative fee %f exceeded the fee budget %f, skipping...", fee.Float64(), budget.Float64())
				return false
			}
		}
	}

	return true
}

func (s *Strategy) handleTradeUpdate(trade types.Trade) {
	log.Infof("received trade %+v", trade)

	if trade.Symbol != s.Symbol {
		return
	}

	if s.state.IsOver24Hours() {
		s.state.Reset()
	}

	// safe check
	if s.state.AccumulatedFees == nil {
		s.state.AccumulatedFees = make(map[string]fixedpoint.Value)
	}

	s.state.AccumulatedFees[trade.FeeCurrency] += fixedpoint.NewFromFloat(trade.Fee)
	s.state.AccumulatedVolume += fixedpoint.NewFromFloat(trade.Quantity)
	log.Infof("accumulated fee: %f %s", s.state.AccumulatedFees[trade.FeeCurrency].Float64(), trade.FeeCurrency)
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	sourceSession, ok := sessions[s.SourceExchange]
	if !ok {
		panic(fmt.Errorf("source session %s is not defined", s.SourceExchange))
	}

	sourceSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
	sourceSession.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{Depth: "5"})

	tradingSession, ok := sessions[s.TradingExchange]
	if !ok {
		panic(fmt.Errorf("trading session %s is not defined", s.TradingExchange))
	}

	tradingSession.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
}

func (s *Strategy) CrossRun(ctx context.Context, _ bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	if s.UpdateInterval == 0 {
		s.UpdateInterval = types.Duration(time.Second)
	}

	sourceSession, ok := sessions[s.SourceExchange]
	if !ok {
		return fmt.Errorf("source session %s is not defined", s.SourceExchange)
	}
	s.sourceSession = sourceSession

	tradingSession, ok := sessions[s.TradingExchange]
	if !ok {
		return fmt.Errorf("trading session %s is not defined", s.TradingExchange)
	}
	s.tradingSession = tradingSession

	s.sourceMarket, ok = s.sourceSession.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("source session market %s is not defined", s.Symbol)
	}

	s.tradingMarket, ok = s.tradingSession.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("trading session market %s is not defined", s.Symbol)
	}

	s.stopC = make(chan struct{})

	var state State
	// load position
	if err := s.Persistence.Load(&state, ID, stateKey); err != nil {
		if err != service.ErrPersistenceNotExists {
			return err
		}

		s.state = &State{}
		s.state.Reset()
	} else {
		// loaded successfully
		s.state = &state
		log.Infof("state is restored: %+v", s.state)

		if s.state.IsOver24Hours() {
			log.Warn("state is over 24 hours, resetting to zero")
			s.state.Reset()
		}
	}

	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		close(s.stopC)

		if err := s.Persistence.Save(&s.state, ID, stateKey); err != nil {
			log.WithError(err).Errorf("can not save state: %+v", s.state)
		} else {
			log.Infof("state is saved => %+v", s.state)
		}
	})

	// from here, set data binding
	s.sourceSession.MarketDataStream.OnKLine(func(kline types.KLine) {
		log.Infof("source exchange %s price: %f volume: %f", s.Symbol, kline.Close, kline.Volume)
		s.mu.Lock()
		s.lastSourceKLine = kline
		s.mu.Unlock()
	})
	s.tradingSession.MarketDataStream.OnKLine(func(kline types.KLine) {
		log.Infof("trading exchange %s price: %f volume: %f", s.Symbol, kline.Close, kline.Volume)
		s.mu.Lock()
		s.lastTradingKLine = kline
		s.mu.Unlock()
	})

	s.sourceBook = types.NewStreamBook(s.Symbol)
	s.sourceBook.BindStream(s.sourceSession.MarketDataStream)

	s.tradingBook = types.NewStreamBook(s.Symbol)
	s.tradingBook.BindStream(s.tradingSession.MarketDataStream)

	s.tradingSession.UserDataStream.OnTradeUpdate(s.handleTradeUpdate)

	instanceID := fmt.Sprintf("%s-%s", ID, s.Symbol)
	s.groupID = max.GenerateGroupID(instanceID)
	log.Infof("using group id %d from fnv32(%s)", s.groupID, instanceID)

	go func() {
		ticker := time.NewTicker(s.UpdateInterval.Duration())
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-s.stopC:
				return

			case <-ticker.C:
				if !s.isBudgetAllowed() {
					continue
				}

				sourceBook := s.sourceBook.Copy()
				book := s.tradingBook.Copy()
				bestBid, hasBid := book.BestBid()
				bestAsk, hasAsk := book.BestAsk()

				// try to use the bid/ask price from the trading book
				if hasBid && hasAsk {
					var spread = bestAsk.Price - bestBid.Price
					var spreadPercentage = spread.Float64() / bestAsk.Price.Float64()
					log.Infof("trading book spread=%f %f%%", spread.Float64(), spreadPercentage*100.0)

					// use the source book price if the spread percentage greater than 10%
					if spreadPercentage > 0.05 {
						log.Warnf("spread too large (%f %f%%), using source book", spread.Float64(), spreadPercentage)
						bestBid, hasBid = sourceBook.BestBid()
						bestAsk, hasAsk = sourceBook.BestAsk()
					}

					// if the spread is less than 100 ticks (100 pips), skip
					if spread.Float64() < 100*s.tradingMarket.TickSize {
						log.Warnf("spread too small, we can't place orders: spread=%f bid=%f ask=%f", spread.Float64(), bestBid.Price.Float64(), bestAsk.Price.Float64())
						continue
					}

				} else {
					bestBid, hasBid = sourceBook.BestBid()
					bestAsk, hasAsk = sourceBook.BestAsk()
				}

				if !hasBid || !hasAsk {
					log.Warn("no bids or asks on the source book or the trading book")
					continue
				}

				var spread = bestAsk.Price - bestBid.Price
				var spreadPercentage = spread.Float64() / bestAsk.Price.Float64()
				log.Infof("spread=%f %f%% ask=%f bid=%f", spread.Float64(), spreadPercentage*100.0, bestAsk.Price.Float64(), bestBid.Price.Float64())
				// var spreadPercentage = spread.Float64() / bestBid.Price.Float64()

				var midPrice = (bestAsk.Price + bestBid.Price).Div(fixedpoint.NewFromFloat(2))
				var price = midPrice.Float64()

				log.Infof("mid price %f", midPrice.Float64())

				var balances = s.tradingSession.Account.Balances()
				var quantity = s.tradingMarket.MinQuantity

				if s.Quantity > 0 {
					quantity = s.Quantity.Float64()
					quantity = math.Min(quantity, s.tradingMarket.MinQuantity)
				} else if s.SimulateVolume {
					s.mu.Lock()
					if s.lastTradingKLine.Volume > 0 && s.lastSourceKLine.Volume > 0 {
						volumeDiff := s.lastSourceKLine.Volume - s.lastTradingKLine.Volume
						// change the current quantity only diff is positive
						if volumeDiff > 0 {
							quantity = volumeDiff
						}

						if baseBalance, ok := balances[s.tradingMarket.BaseCurrency]; ok {
							quantity = math.Min(quantity, baseBalance.Available.Float64())
						}

						if quoteBalance, ok := balances[s.tradingMarket.QuoteCurrency]; ok {
							maxQuantity := quoteBalance.Available.Float64() / price
							quantity = math.Min(quantity, maxQuantity)
						}
					}
					s.mu.Unlock()
				}

				var quoteAmount = price * quantity
				if quoteAmount <= s.tradingMarket.MinNotional {
					quantity = math.Max(
						s.tradingMarket.MinQuantity,
						s.tradingMarket.MinNotional*1.01/price)
				}

				createdOrders, err := tradingSession.Exchange.SubmitOrders(ctx, types.SubmitOrder{
					Symbol:   s.Symbol,
					Side:     types.SideTypeBuy,
					Type:     types.OrderTypeLimit,
					Quantity: quantity,
					Price:    price,
					Market:   s.tradingMarket,
					// TimeInForce: "GTC",
					GroupID: s.groupID,
				}, types.SubmitOrder{
					Symbol:   s.Symbol,
					Side:     types.SideTypeSell,
					Type:     types.OrderTypeLimit,
					Quantity: quantity,
					Price:    price,
					Market:   s.tradingMarket,
					// TimeInForce: "GTC",
					GroupID: s.groupID,
				})
				if err != nil {
					log.WithError(err).Error("order submit error")
				}

				time.Sleep(time.Second)

				if err := tradingSession.Exchange.CancelOrders(ctx, createdOrders...); err != nil {
					log.WithError(err).Error("cancel order error")
				}
			}
		}
	}()

	return nil
}

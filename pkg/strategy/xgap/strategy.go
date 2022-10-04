package xgap

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

const ID = "xgap"

const stateKey = "state-v1"

var log = logrus.WithField("strategy", ID)

var StepPercentageGap = fixedpoint.NewFromFloat(0.05)
var NotionModifier = fixedpoint.NewFromFloat(1.01)
var Two = fixedpoint.NewFromInt(2)

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
	return time.Since(s.AccumulatedFeeStartedAt) >= 24*time.Hour
}

func (s *State) Reset() {
	t := time.Now()
	dateTime := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())

	log.Infof("resetting accumulated started time to: %s", dateTime)

	s.AccumulatedFeeStartedAt = dateTime
	s.AccumulatedFees = make(map[string]fixedpoint.Value)
	s.AccumulatedVolume = fixedpoint.Zero
}

type Strategy struct {
	Symbol          string           `json:"symbol"`
	SourceExchange  string           `json:"sourceExchange"`
	TradingExchange string           `json:"tradingExchange"`
	MinSpread       fixedpoint.Value `json:"minSpread"`
	Quantity        fixedpoint.Value `json:"quantity"`

	DailyFeeBudgets map[string]fixedpoint.Value `json:"dailyFeeBudgets,omitempty"`
	DailyMaxVolume  fixedpoint.Value            `json:"dailyMaxVolume,omitempty"`
	UpdateInterval  types.Duration              `json:"updateInterval"`
	SimulateVolume  bool                        `json:"simulateVolume"`

	sourceSession, tradingSession *bbgo.ExchangeSession
	sourceMarket, tradingMarket   types.Market

	State *State `persistence:"state"`

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

	if s.State.AccumulatedFees == nil {
		return true
	}

	for asset, budget := range s.DailyFeeBudgets {
		if fee, ok := s.State.AccumulatedFees[asset]; ok {
			if fee.Compare(budget) >= 0 {
				log.Warnf("accumulative fee %s exceeded the fee budget %s, skipping...", fee.String(), budget.String())
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

	if s.State.IsOver24Hours() {
		s.State.Reset()
	}

	// safe check
	if s.State.AccumulatedFees == nil {
		s.State.AccumulatedFees = make(map[string]fixedpoint.Value)
	}

	s.State.AccumulatedFees[trade.FeeCurrency] = s.State.AccumulatedFees[trade.FeeCurrency].Add(trade.Fee)
	s.State.AccumulatedVolume = s.State.AccumulatedVolume.Add(trade.Quantity)
	log.Infof("accumulated fee: %s %s", s.State.AccumulatedFees[trade.FeeCurrency].String(), trade.FeeCurrency)
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	sourceSession, ok := sessions[s.SourceExchange]
	if !ok {
		panic(fmt.Errorf("source session %s is not defined", s.SourceExchange))
	}

	sourceSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
	sourceSession.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{})

	tradingSession, ok := sessions[s.TradingExchange]
	if !ok {
		panic(fmt.Errorf("trading session %s is not defined", s.TradingExchange))
	}

	tradingSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
	tradingSession.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{})
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

	if s.State == nil {
		s.State = &State{}
		s.State.Reset()
	}

	if s.State.IsOver24Hours() {
		log.Warn("state is over 24 hours, resetting to zero")
		s.State.Reset()
	}

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		close(s.stopC)
		bbgo.Sync(context.Background(), s)
	})

	// from here, set data binding
	s.sourceSession.MarketDataStream.OnKLine(func(kline types.KLine) {
		log.Infof("source exchange %s price: %s volume: %s",
			s.Symbol, kline.Close.String(), kline.Volume.String())
		s.mu.Lock()
		s.lastSourceKLine = kline
		s.mu.Unlock()
	})
	s.tradingSession.MarketDataStream.OnKLine(func(kline types.KLine) {
		log.Infof("trading exchange %s price: %s volume: %s",
			s.Symbol, kline.Close.String(), kline.Volume.String())
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
	s.groupID = util.FNV32(instanceID)
	log.Infof("using group id %d from fnv32(%s)", s.groupID, instanceID)

	go func() {
		ticker := time.NewTicker(
			util.MillisecondsJitter(s.UpdateInterval.Duration(), 1000),
		)
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

				// < 10 seconds jitter sleep
				delay := util.MillisecondsJitter(s.UpdateInterval.Duration(), 10*1000)
				if delay < s.UpdateInterval.Duration() {
					time.Sleep(delay)
				}

				bestBid, hasBid := s.tradingBook.BestBid()
				bestAsk, hasAsk := s.tradingBook.BestAsk()

				// try to use the bid/ask price from the trading book
				if hasBid && hasAsk {
					var spread = bestAsk.Price.Sub(bestBid.Price)
					var spreadPercentage = spread.Div(bestAsk.Price)
					log.Infof("trading book spread=%s %s",
						spread.String(), spreadPercentage.Percentage())

					// use the source book price if the spread percentage greater than 10%
					if spreadPercentage.Compare(StepPercentageGap) > 0 {
						log.Warnf("spread too large (%s %s), using source book",
							spread.String(), spreadPercentage.Percentage())
						bestBid, hasBid = s.sourceBook.BestBid()
						bestAsk, hasAsk = s.sourceBook.BestAsk()
					}

					if s.MinSpread.Sign() > 0 {
						if spread.Compare(s.MinSpread) < 0 {
							log.Warnf("spread < min spread, spread=%s minSpread=%s bid=%s ask=%s",
								spread.String(), s.MinSpread.String(),
								bestBid.Price.String(), bestAsk.Price.String())
							continue
						}
					}

					// if the spread is less than 100 ticks (100 pips), skip
					if spread.Compare(s.tradingMarket.TickSize.MulExp(2)) < 0 {
						log.Warnf("spread too small, we can't place orders: spread=%v bid=%v ask=%v",
							spread, bestBid.Price, bestAsk.Price)
						continue
					}

				} else {
					bestBid, hasBid = s.sourceBook.BestBid()
					bestAsk, hasAsk = s.sourceBook.BestAsk()
				}

				if !hasBid || !hasAsk {
					log.Warn("no bids or asks on the source book or the trading book")
					continue
				}

				var spread = bestAsk.Price.Sub(bestBid.Price)
				var spreadPercentage = spread.Div(bestAsk.Price)
				log.Infof("spread=%v %s ask=%v bid=%v",
					spread, spreadPercentage.Percentage(),
					bestAsk.Price, bestBid.Price)
				// var spreadPercentage = spread.Float64() / bestBid.Price.Float64()

				var midPrice = bestAsk.Price.Add(bestBid.Price).Div(Two)
				var price = midPrice

				log.Infof("mid price %v", midPrice)

				var balances = s.tradingSession.GetAccount().Balances()
				var quantity = s.tradingMarket.MinQuantity

				if s.Quantity.Sign() > 0 {
					quantity = fixedpoint.Min(s.Quantity, s.tradingMarket.MinQuantity)
				} else if s.SimulateVolume {
					s.mu.Lock()
					if s.lastTradingKLine.Volume.Sign() > 0 && s.lastSourceKLine.Volume.Sign() > 0 {
						volumeDiff := s.lastSourceKLine.Volume.Sub(s.lastTradingKLine.Volume)
						// change the current quantity only diff is positive
						if volumeDiff.Sign() > 0 {
							quantity = volumeDiff
						}

						if baseBalance, ok := balances[s.tradingMarket.BaseCurrency]; ok {
							quantity = fixedpoint.Min(quantity, baseBalance.Available)
						}

						if quoteBalance, ok := balances[s.tradingMarket.QuoteCurrency]; ok {
							maxQuantity := quoteBalance.Available.Div(price)
							quantity = fixedpoint.Min(quantity, maxQuantity)
						}
					}
					s.mu.Unlock()
				} else {
					// plus a 2% quantity jitter
					jitter := 1.0 + math.Max(0.02, rand.Float64())
					quantity = quantity.Mul(fixedpoint.NewFromFloat(jitter))
				}

				var quoteAmount = price.Mul(quantity)
				if quoteAmount.Compare(s.tradingMarket.MinNotional) <= 0 {
					quantity = fixedpoint.Max(
						s.tradingMarket.MinQuantity,
						s.tradingMarket.MinNotional.Mul(NotionModifier).Div(price))
				}

				createdOrders, _, err := bbgo.BatchPlaceOrder(ctx, tradingSession.Exchange, types.SubmitOrder{
					Symbol:   s.Symbol,
					Side:     types.SideTypeBuy,
					Type:     types.OrderTypeLimit,
					Quantity: quantity,
					Price:    price,
					Market:   s.tradingMarket,
					// TimeInForce: types.TimeInForceGTC,
					GroupID: s.groupID,
				}, types.SubmitOrder{
					Symbol:   s.Symbol,
					Side:     types.SideTypeSell,
					Type:     types.OrderTypeLimit,
					Quantity: quantity,
					Price:    price,
					Market:   s.tradingMarket,
					// TimeInForce: types.TimeInForceGTC,
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

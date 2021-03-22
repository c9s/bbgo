package gap

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "xkline"

const stateKey = "state-v1"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

func (s *Strategy) ID() string {
	return ID
}

type State struct{}

type Strategy struct {
	*bbgo.Graceful
	*bbgo.Notifiability
	*bbgo.Persistence

	Symbol          string `json:"symbol"`
	SourceExchange  string `json:"sourceExchange"`
	TradingExchange string `json:"tradingExchange"`

	DailyFeeBudget fixedpoint.Value `json:"dailyFeeBudget"`
	UpdateInterval types.Duration   `json:"updateInterval"`

	sourceSession, tradingSession *bbgo.ExchangeSession
	sourceMarket, tradingMarket   types.Market

	state *State

	mu                      sync.Mutex
	lastKLine               types.KLine
	sourceBook, tradingBook *types.StreamOrderBook
	groupID                 int64

	stopC chan struct{}
}

func (s *Strategy) handleTradeUpdate(trade types.Trade) {
	log.Infof("received trade %+v", trade)

	if trade.Symbol != s.Symbol {
		return
	}
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

func (s *Strategy) CrossRun(ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
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

	// from here, set data binding
	s.sourceSession.Stream.OnKLine(func(kline types.KLine) {
		log.Infof("source exchange %s price: %f", s.Symbol, kline.Close)
		s.mu.Lock()
		s.lastKLine = kline
		s.mu.Unlock()
	})

	s.sourceBook = types.NewStreamBook(s.Symbol)
	s.sourceBook.BindStream(s.sourceSession.Stream)

	s.tradingBook = types.NewStreamBook(s.Symbol)
	s.tradingBook.BindStream(s.tradingSession.Stream)

	instanceID := fmt.Sprintf("%s-%s", ID, s.Symbol)
	s.groupID = generateGroupID(instanceID)
	log.Infof("using group id %d from fnv32(%s)", s.groupID, instanceID)

	s.tradingSession.Stream.OnTradeUpdate(s.handleTradeUpdate)

	go func() {
		ticker := time.NewTicker(s.UpdateInterval.Duration())
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				sourceBook := s.sourceBook.Get()
				book := s.tradingBook.Get()
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

					// if the spread is less than 10 ticks (10 pips), skip
					if spread.Float64() < 10*s.tradingMarket.TickSize {
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

				var quantity = s.tradingMarket.MinQuantity
				var quoteAmount = price * quantity
				if quoteAmount <= s.tradingMarket.MinNotional {
					quantity = math.Max(
						s.tradingMarket.MinQuantity,
						s.tradingMarket.MinNotional*1.01/price)
				}

				createdOrders, err := tradingSession.Exchange.SubmitOrders(ctx, types.SubmitOrder{
					Symbol:      s.Symbol,
					Side:        types.SideTypeBuy,
					Type:        types.OrderTypeLimit,
					Quantity:    quantity,
					Price:       price,
					Market:      s.tradingMarket,
					TimeInForce: "GTC",
					GroupID:     s.groupID,
				}, types.SubmitOrder{
					Symbol:      s.Symbol,
					Side:        types.SideTypeSell,
					Type:        types.OrderTypeLimit,
					Quantity:    quantity,
					Price:       price,
					Market:      s.tradingMarket,
					TimeInForce: "GTC",
					GroupID:     s.groupID,
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

func generateGroupID(s string) int64 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return int64(h.Sum32())
}

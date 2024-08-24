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
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

const ID = "xgap"

var log = logrus.WithField("strategy", ID)

var maxStepPercentageGap = fixedpoint.NewFromFloat(0.05)

var Two = fixedpoint.NewFromInt(2)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

type Strategy struct {
	*common.Strategy
	*common.FeeBudget

	Environment *bbgo.Environment

	Symbol          string           `json:"symbol"`
	SourceExchange  string           `json:"sourceExchange"`
	TradingExchange string           `json:"tradingExchange"`
	MinSpread       fixedpoint.Value `json:"minSpread"`
	Quantity        fixedpoint.Value `json:"quantity"`
	DryRun          bool             `json:"dryRun"`

	DailyMaxVolume    fixedpoint.Value `json:"dailyMaxVolume,omitempty"`
	DailyTargetVolume fixedpoint.Value `json:"dailyTargetVolume,omitempty"`
	UpdateInterval    types.Duration   `json:"updateInterval"`
	SimulateVolume    bool             `json:"simulateVolume"`
	SimulatePrice     bool             `json:"simulatePrice"`

	sourceSession, tradingSession *bbgo.ExchangeSession
	sourceMarket, tradingMarket   types.Market

	mu                                sync.Mutex
	lastSourceKLine, lastTradingKLine types.KLine
	sourceBook, tradingBook           *types.StreamOrderBook

	stopC chan struct{}
}

func (s *Strategy) Initialize() error {
	if s.Strategy == nil {
		s.Strategy = &common.Strategy{}
	}

	if s.FeeBudget == nil {
		s.FeeBudget = &common.FeeBudget{}
	}
	return nil
}

func (s *Strategy) Validate() error {
	return nil
}

func (s *Strategy) Defaults() error {
	if s.UpdateInterval == 0 {
		s.UpdateInterval = types.Duration(time.Second)
	}
	return nil
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	sourceSession, ok := sessions[s.SourceExchange]
	if !ok {
		panic(fmt.Errorf("source session %s is not defined", s.SourceExchange))
	}

	sourceSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
	sourceSession.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{Depth: types.DepthLevel5})

	tradingSession, ok := sessions[s.TradingExchange]
	if !ok {
		panic(fmt.Errorf("trading session %s is not defined", s.TradingExchange))
	}

	tradingSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
	tradingSession.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{Depth: types.DepthLevel5})
}

func (s *Strategy) CrossRun(ctx context.Context, _ bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
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

	s.Strategy.Initialize(ctx, s.Environment, tradingSession, s.tradingMarket, ID, s.InstanceID())
	s.FeeBudget.Initialize()

	s.stopC = make(chan struct{})

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		close(s.stopC)
		bbgo.Sync(ctx, s)
	})

	// from here, set data binding
	s.sourceSession.MarketDataStream.OnKLine(func(kline types.KLine) {
		s.mu.Lock()
		s.lastSourceKLine = kline
		s.mu.Unlock()
	})
	s.tradingSession.MarketDataStream.OnKLine(func(kline types.KLine) {
		s.mu.Lock()
		s.lastTradingKLine = kline
		s.mu.Unlock()
	})

	if s.SourceExchange != "" {
		s.sourceBook = types.NewStreamBook(s.Symbol, sourceSession.ExchangeName)
		s.sourceBook.BindStream(s.sourceSession.MarketDataStream)
	}

	s.tradingBook = types.NewStreamBook(s.Symbol, tradingSession.ExchangeName)
	s.tradingBook.BindStream(s.tradingSession.MarketDataStream)

	s.tradingSession.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
		if trade.Symbol != s.Symbol {
			return
		}
		s.FeeBudget.HandleTradeUpdate(trade)
	})

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
				if !s.IsBudgetAllowed() {
					continue
				}

				// < 10 seconds jitter sleep
				delay := util.MillisecondsJitter(s.UpdateInterval.Duration(), 10*1000)
				if delay < s.UpdateInterval.Duration() {
					time.Sleep(delay)
				}

				s.placeOrders(ctx)

				s.cancelOrders(ctx)
			}
		}
	}()

	return nil
}

func (s *Strategy) placeOrders(ctx context.Context) {
	bestBid, hasBid := s.tradingBook.BestBid()
	bestAsk, hasAsk := s.tradingBook.BestAsk()

	// try to use the bid/ask price from the trading book
	if hasBid && hasAsk {
		var spread = bestAsk.Price.Sub(bestBid.Price)
		var spreadPercentage = spread.Div(bestAsk.Price)
		log.Infof("trading book spread=%s %s",
			spread.String(), spreadPercentage.Percentage())

		// use the source book price if the spread percentage greater than 5%
		if s.SimulatePrice && s.sourceBook != nil && spreadPercentage.Compare(maxStepPercentageGap) > 0 {
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
				return
			}
		}

		// if the spread is less than 100 ticks (100 pips), skip
		if spread.Compare(s.tradingMarket.TickSize.MulExp(2)) < 0 {
			log.Warnf("spread too small, we can't place orders: spread=%s bid=%s ask=%s",
				spread.String(), bestBid.Price.String(), bestAsk.Price.String())
			return
		}

	} else if s.sourceBook != nil {
		bestBid, hasBid = s.sourceBook.BestBid()
		bestAsk, hasAsk = s.sourceBook.BestAsk()
	}

	if !hasBid || !hasAsk {
		log.Warn("no bids or asks on the source book or the trading book")
		return
	}

	if bestBid.Price.IsZero() || bestAsk.Price.IsZero() {
		log.Warn("bid price or ask price is zero")
		return
	}

	var spread = bestAsk.Price.Sub(bestBid.Price)
	var spreadPercentage = spread.Div(bestAsk.Price)

	log.Infof("spread:%s %s ask:%s bid:%s",
		spread.String(), spreadPercentage.Percentage(),
		bestAsk.Price.String(), bestBid.Price.String())
	// var spreadPercentage = spread.Float64() / bestBid.Price.Float64()

	var midPrice = bestAsk.Price.Add(bestBid.Price).Div(Two)
	var price = midPrice

	log.Infof("mid price %s", midPrice.String())

	var balances = s.tradingSession.GetAccount().Balances()

	baseBalance, ok := balances[s.tradingMarket.BaseCurrency]
	if !ok {
		log.Errorf("base balance %s not found", s.tradingMarket.BaseCurrency)
		return
	}

	quoteBalance, ok := balances[s.tradingMarket.QuoteCurrency]
	if !ok {
		log.Errorf("quote balance %s not found", s.tradingMarket.QuoteCurrency)
		return
	}

	minQuantity := s.tradingMarket.AdjustQuantityByMinNotional(s.tradingMarket.MinQuantity, price)

	if baseBalance.Available.Compare(minQuantity) <= 0 {
		log.Infof("base balance: %s %s is not enough, skip", baseBalance.Available.String(), s.tradingMarket.BaseCurrency)
		return
	}

	if quoteBalance.Available.Div(price).Compare(minQuantity) <= 0 {
		log.Infof("quote balance: %s %s is not enough, skip", quoteBalance.Available.String(), s.tradingMarket.QuoteCurrency)
		return
	}

	maxQuantity := baseBalance.Available
	if !quoteBalance.Available.IsZero() {
		maxQuantity = fixedpoint.Min(maxQuantity, quoteBalance.Available.Div(price))
	}

	quantity := minQuantity

	// if we set the fixed quantity, we should use the fixed
	if s.Quantity.Sign() > 0 {
		quantity = fixedpoint.Max(s.Quantity, quantity)
	} else if s.SimulateVolume {
		s.mu.Lock()
		if s.lastTradingKLine.Volume.Sign() > 0 && s.lastSourceKLine.Volume.Sign() > 0 {
			log.Infof("trading exchange %s price: %s volume: %s",
				s.Symbol, s.lastTradingKLine.Close.String(), s.lastTradingKLine.Volume.String())
			log.Infof("source exchange %s price: %s volume: %s",
				s.Symbol, s.lastSourceKLine.Close.String(), s.lastSourceKLine.Volume.String())

			volumeDiff := s.lastSourceKLine.Volume.Sub(s.lastTradingKLine.Volume)
			// change the current quantity only diff is positive
			if volumeDiff.Sign() > 0 {
				quantity = volumeDiff
			}
		}
		s.mu.Unlock()
	} else if s.DailyTargetVolume.Sign() > 0 {
		numOfTicks := (24 * time.Hour) / s.UpdateInterval.Duration()
		quantity = fixedpoint.NewFromFloat(s.DailyTargetVolume.Float64() / float64(numOfTicks))
		quantity = quantityJitter(quantity, 0.02)
	} else {
		// plus a 2% quantity jitter
		quantity = quantityJitter(quantity, 0.02)
	}

	log.Infof("%s quantity: %f", s.Symbol, quantity.Float64())

	quantity = fixedpoint.Min(quantity, maxQuantity)

	log.Infof("%s adjusted quantity: %f", s.Symbol, quantity.Float64())

	orderForms := []types.SubmitOrder{
		{
			Symbol:   s.Symbol,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Quantity: quantity,
			Price:    price,
			Market:   s.tradingMarket,
		},
		{
			Symbol:   s.Symbol,
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeLimit,
			Quantity: quantity,
			Price:    price,
			Market:   s.tradingMarket,
		},
	}
	log.Infof("order forms: %+v", orderForms)

	if s.DryRun {
		log.Infof("dry run, skip")
		return
	}

	_, err := s.OrderExecutor.SubmitOrders(ctx, orderForms...)
	if err != nil {
		log.WithError(err).Error("order submit error")
	}

	time.Sleep(time.Second)
}

func (s *Strategy) cancelOrders(ctx context.Context) {
	if err := s.OrderExecutor.GracefulCancel(ctx); err != nil {
		log.WithError(err).Error("cancel order error")
	}
}

func quantityJitter(q fixedpoint.Value, rg float64) fixedpoint.Value {
	jitter := 1.0 + math.Max(rg, rand.Float64())
	return q.Mul(fixedpoint.NewFromFloat(jitter))
}

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
	"github.com/c9s/bbgo/pkg/util/timejitter"
	"github.com/c9s/bbgo/pkg/util/tradingutil"
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
	return fmt.Sprintf("%s:%s:%s", ID, s.TradingExchange, s.Symbol)
}

type Strategy struct {
	*common.Strategy
	*common.FeeBudget

	Environment *bbgo.Environment

	Symbol          string `json:"symbol"`
	TradingExchange string `json:"tradingExchange"`

	SourceSymbol   string `json:"sourceSymbol"`
	SourceExchange string `json:"sourceExchange"`

	MinSpread         fixedpoint.Value `json:"minSpread"`
	MakeSpread        bool             `json:"makeSpread"`
	Quantity          fixedpoint.Value `json:"quantity"`
	MaxJitterQuantity fixedpoint.Value `json:"maxJitterQuantity"`

	DryRun bool `json:"dryRun"`

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

	logger logrus.FieldLogger

	stopC chan struct{}
}

func (s *Strategy) Initialize() error {
	if s.Strategy == nil {
		s.Strategy = &common.Strategy{}
	}

	if s.FeeBudget == nil {
		s.FeeBudget = &common.FeeBudget{}
	}

	s.logger = logrus.WithFields(logrus.Fields{
		"strategy":          ID,
		"strategy_instance": s.InstanceID(),
		"symbol":            s.Symbol,
	})
	return nil
}

func (s *Strategy) Validate() error {
	return nil
}

func (s *Strategy) Defaults() error {
	if s.UpdateInterval == 0 {
		s.UpdateInterval = types.Duration(time.Second)
	}

	if s.SourceSymbol == "" {
		s.SourceSymbol = s.Symbol
	}

	return nil
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	if len(s.SourceExchange) > 0 && len(s.SourceSymbol) > 0 {
		sourceSession, ok := sessions[s.SourceExchange]
		if !ok {
			panic(fmt.Errorf("source session %s is not defined", s.SourceExchange))
		}

		sourceSession.Subscribe(types.KLineChannel, s.SourceSymbol, types.SubscribeOptions{Interval: "1m"})
		sourceSession.Subscribe(types.BookChannel, s.SourceSymbol, types.SubscribeOptions{Depth: types.DepthLevelFull})
	}

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

	s.sourceMarket, ok = s.sourceSession.Market(s.SourceSymbol)
	if !ok {
		return fmt.Errorf("source session market %s is not defined", s.Symbol)
	}

	tradingSession, ok := sessions[s.TradingExchange]
	if !ok {
		return fmt.Errorf("trading session %s is not defined", s.TradingExchange)
	}
	s.tradingSession = tradingSession

	s.tradingMarket, ok = s.tradingSession.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("trading session market %s is not defined", s.Symbol)
	}

	s.Strategy.Initialize(ctx, s.Environment, tradingSession, s.tradingMarket, ID, s.InstanceID())
	s.Strategy.OrderExecutor.DisableNotify()

	s.FeeBudget.Initialize()

	s.stopC = make(chan struct{})

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		close(s.stopC)

		if err := tradingutil.UniversalCancelAllOrders(ctx, s.tradingSession.Exchange, s.Symbol, nil); err != nil {
			s.logger.WithError(err).Errorf("cancel all orders error")
		}

		bbgo.Sync(ctx, s)
	})

	// from here, set data binding
	sourceKLineHandler := func(kline types.KLine) {
		s.mu.Lock()
		s.lastSourceKLine = kline
		s.mu.Unlock()
	}
	s.sourceSession.MarketDataStream.OnKLine(sourceKLineHandler)
	s.tradingSession.MarketDataStream.OnKLine(sourceKLineHandler)

	if s.SourceExchange != "" && s.SourceSymbol != "" {
		s.sourceBook = types.NewStreamBook(s.SourceSymbol, sourceSession.ExchangeName)
		s.sourceBook.BindStream(s.sourceSession.MarketDataStream)
		s.sourceBook.DebugBindStream(s.sourceSession.MarketDataStream, log)
	}

	s.tradingBook = types.NewStreamBook(s.Symbol, tradingSession.ExchangeName)
	s.tradingBook.BindStream(s.tradingSession.MarketDataStream)
	s.tradingBook.DebugBindStream(s.tradingSession.MarketDataStream, log)

	s.tradingSession.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
		if trade.Symbol != s.Symbol {
			return
		}
		s.FeeBudget.HandleTradeUpdate(trade)
	})

	go func() {
		ticker := time.NewTicker(
			timejitter.Milliseconds(s.UpdateInterval.Duration(), 1000),
		)
		defer ticker.Stop()

		s.placeOrders(ctx)
		s.cancelOrders(ctx)

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
				delay := timejitter.Milliseconds(s.UpdateInterval.Duration(), 10*1000)
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

func (s *Strategy) makeSpread(ctx context.Context, bestBid, bestAsk types.PriceVolume) error {
	orderForms := []types.SubmitOrder{
		{
			Symbol:      s.Symbol,
			Side:        types.SideTypeSell,
			Type:        types.OrderTypeLimit,
			Quantity:    fixedpoint.Max(bestBid.Volume, s.tradingMarket.MinQuantity),
			Price:       bestBid.Price,
			Market:      s.tradingMarket,
			TimeInForce: types.TimeInForceIOC,
		},
	}
	log.Infof("make spread order forms: %+v", orderForms)

	if s.DryRun {
		log.Infof("dry run, skip")
		return nil
	}

	_, err := s.OrderExecutor.SubmitOrders(ctx, orderForms...)
	return err
}

func (s *Strategy) placeOrders(ctx context.Context) {
	bestBid, hasBid := s.tradingBook.BestBid()
	bestAsk, hasAsk := s.tradingBook.BestAsk()

	// try to use the bid/ask price from the trading book
	if hasBid && hasAsk {
		var spread = bestAsk.Price.Sub(bestBid.Price)
		var spreadPercentage = spread.Div(bestAsk.Price)
		log.Infof("trading book spread=%s(%s-%s) %s",
			spread.String(), bestAsk.Price.String(), bestBid.Price.String(), spreadPercentage.Percentage())

		// use the source book price if the spread percentage greater than 5%
		if s.SimulatePrice && s.sourceBook != nil && spreadPercentage.Compare(maxStepPercentageGap) > 0 {
			log.Warnf("spread too large (%s %s), using source book",
				spread.String(), spreadPercentage.Percentage())
			bestBid, hasBid = s.sourceBook.BestBid()
			bestAsk, hasAsk = s.sourceBook.BestAsk()
		}

		if s.MinSpread.Sign() > 0 {
			if spreadPercentage.Compare(s.MinSpread) < 0 {
				log.Warnf("spread < min spread, spread=%s minSpread=%s bid=%s ask=%s",
					spreadPercentage.String(), s.MinSpread.String(),
					bestBid.Price.String(), bestAsk.Price.String())

				if s.MakeSpread {
					if err := s.makeSpread(ctx, bestBid, bestAsk); err != nil {
						log.WithError(err).Error("make spread error")
					}
				}
				return
			}
		}

		// if the spread is less than 2 ticks, skip
		if spread.Compare(s.tradingMarket.TickSize.Mul(fixedpoint.NewFromFloat(2.0))) < 0 {
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
		lastTradingKLine := s.lastTradingKLine
		lastSourceKLine := s.lastSourceKLine
		s.mu.Unlock()

		if lastTradingKLine.Volume.Sign() > 0 && lastSourceKLine.Volume.Sign() > 0 {
			log.Infof("trading exchange %s price: %s volume: %s",
				s.Symbol, lastTradingKLine.Close.String(), lastTradingKLine.Volume.String())
			log.Infof("source exchange %s price: %s volume: %s",
				s.Symbol, lastSourceKLine.Close.String(), lastSourceKLine.Volume.String())

			volumeDiff := s.lastSourceKLine.Volume.Sub(lastTradingKLine.Volume)

			// change the current quantity only diff is positive
			if volumeDiff.Sign() > 0 {
				quantity = volumeDiff
			}
		}
	} else if s.DailyTargetVolume.Sign() > 0 {
		numOfTicks := (24 * time.Hour) / s.UpdateInterval.Duration()
		quantity = fixedpoint.NewFromFloat(s.DailyTargetVolume.Float64() / float64(numOfTicks))
	}

	if s.MaxJitterQuantity.Sign() > 0 {
		quantity = quantityJitter2(quantity, s.MaxJitterQuantity)
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
			Symbol:      s.Symbol,
			Side:        types.SideTypeSell,
			Type:        types.OrderTypeLimit,
			Quantity:    quantity,
			Price:       price,
			Market:      s.tradingMarket,
			TimeInForce: types.TimeInForceIOC,
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

	if err := s.OrderExecutor.GracefulCancel(ctx); err != nil {
		log.WithError(err).Warnf("cancel order error")
	}
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

func quantityJitter2(q, maxJitterQ fixedpoint.Value) fixedpoint.Value {
	rg := maxJitterQ.Sub(q).Float64()
	randQuantity := fixedpoint.NewFromFloat(q.Float64() + rg*rand.Float64())
	return randQuantity
}

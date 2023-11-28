package xdepthmaker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

var lastPriceModifier = fixedpoint.NewFromFloat(1.001)
var minGap = fixedpoint.NewFromFloat(1.02)
var defaultMargin = fixedpoint.NewFromFloat(0.003)

var Two = fixedpoint.NewFromInt(2)

const priceUpdateTimeout = 30 * time.Second

const ID = "xdepthmaker"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

func notifyTrade(trade types.Trade, _, _ fixedpoint.Value) {
	bbgo.Notify(trade)
}

type CrossExchangeMarketMakingStrategy struct {
	ctx, parent context.Context
	cancel      context.CancelFunc

	Environ *bbgo.Environment

	makerSession, hedgeSession *bbgo.ExchangeSession
	makerMarket, hedgeMarket   types.Market

	Position    *types.Position    `json:"position,omitempty" persistence:"position"`
	ProfitStats *types.ProfitStats `json:"profitStats,omitempty" persistence:"profit_stats"`

	MakerOrderExecutor, HedgeOrderExecutor *bbgo.GeneralOrderExecutor

	// orderStore is a shared order store between the maker session and the hedge session
	orderStore *core.OrderStore

	// tradeCollector is a shared trade collector between the maker session and the hedge session
	tradeCollector *core.TradeCollector
}

func (s *CrossExchangeMarketMakingStrategy) Initialize(
	ctx context.Context, environ *bbgo.Environment,
	makerSession, hedgeSession *bbgo.ExchangeSession,
	symbol, strategyID, instanceID string,
) error {
	s.parent = ctx
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.Environ = environ

	s.makerSession = makerSession
	s.hedgeSession = hedgeSession

	var ok bool
	s.hedgeMarket, ok = s.hedgeSession.Market(symbol)
	if !ok {
		return fmt.Errorf("source session market %s is not defined", symbol)
	}

	s.makerMarket, ok = s.makerSession.Market(symbol)
	if !ok {
		return fmt.Errorf("maker session market %s is not defined", symbol)
	}

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.makerMarket)
	}

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.makerMarket)
	}

	// Always update the position fields
	s.Position.Strategy = strategyID
	s.Position.StrategyInstanceID = instanceID

	// if anyone of the fee rate is defined, this assumes that both are defined.
	// so that zero maker fee could be applied
	for _, ses := range []*bbgo.ExchangeSession{makerSession, hedgeSession} {
		if ses.MakerFeeRate.Sign() > 0 || ses.TakerFeeRate.Sign() > 0 {
			s.Position.SetExchangeFeeRate(ses.ExchangeName, types.ExchangeFee{
				MakerFeeRate: ses.MakerFeeRate,
				TakerFeeRate: ses.TakerFeeRate,
			})
		}
	}

	s.MakerOrderExecutor = bbgo.NewGeneralOrderExecutor(
		makerSession,
		s.makerMarket.Symbol,
		strategyID, instanceID,
		s.Position)
	s.MakerOrderExecutor.BindEnvironment(environ)
	s.MakerOrderExecutor.BindProfitStats(s.ProfitStats)
	s.MakerOrderExecutor.Bind()
	s.MakerOrderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		// bbgo.Sync(ctx, s)
	})

	s.HedgeOrderExecutor = bbgo.NewGeneralOrderExecutor(
		hedgeSession,
		s.hedgeMarket.Symbol,
		strategyID, instanceID,
		s.Position)
	s.HedgeOrderExecutor.BindEnvironment(environ)
	s.HedgeOrderExecutor.BindProfitStats(s.ProfitStats)
	s.HedgeOrderExecutor.Bind()
	s.HedgeOrderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		// bbgo.Sync(ctx, s)
	})

	s.orderStore = core.NewOrderStore(s.Position.Symbol)
	s.orderStore.BindStream(hedgeSession.UserDataStream)
	s.orderStore.BindStream(makerSession.UserDataStream)
	s.tradeCollector = core.NewTradeCollector(s.Position.Symbol, s.Position, s.orderStore)

	return nil
}

type Strategy struct {
	*CrossExchangeMarketMakingStrategy

	Environment *bbgo.Environment

	Symbol string `json:"symbol"`

	// HedgeExchange session name
	HedgeExchange string `json:"hedgeExchange"`

	// MakerExchange session name
	MakerExchange string `json:"makerExchange"`

	UpdateInterval      types.Duration `json:"updateInterval"`
	HedgeInterval       types.Duration `json:"hedgeInterval"`
	OrderCancelWaitTime types.Duration `json:"orderCancelWaitTime"`

	Margin        fixedpoint.Value `json:"margin"`
	BidMargin     fixedpoint.Value `json:"bidMargin"`
	AskMargin     fixedpoint.Value `json:"askMargin"`
	UseDepthPrice bool             `json:"useDepthPrice"`
	DepthQuantity fixedpoint.Value `json:"depthQuantity"`

	StopHedgeQuoteBalance fixedpoint.Value `json:"stopHedgeQuoteBalance"`
	StopHedgeBaseBalance  fixedpoint.Value `json:"stopHedgeBaseBalance"`

	// Quantity is used for fixed quantity of the first layer
	Quantity fixedpoint.Value `json:"quantity"`

	// QuantityScale helps user to define the quantity by layer scale
	QuantityScale *bbgo.LayerScale `json:"quantityScale,omitempty"`

	// MaxExposurePosition defines the unhedged quantity of stop
	MaxExposurePosition fixedpoint.Value `json:"maxExposurePosition"`

	DisableHedge bool `json:"disableHedge"`

	NotifyTrade bool `json:"notifyTrade"`

	// RecoverTrade tries to find the missing trades via the REStful API
	RecoverTrade bool `json:"recoverTrade"`

	RecoverTradeScanPeriod types.Duration `json:"recoverTradeScanPeriod"`

	NumLayers int `json:"numLayers"`

	// Pips is the pips of the layer prices
	Pips fixedpoint.Value `json:"pips"`

	// --------------------------------
	// private fields
	// --------------------------------
	state *State

	// persistence fields
	CoveredPosition fixedpoint.Value `json:"coveredPosition,omitempty" persistence:"covered_position"`

	// pricingBook is the order book (depth) from the hedging session
	pricingBook *types.StreamOrderBook

	activeMakerOrders *bbgo.ActiveOrderBook

	hedgeErrorLimiter         *rate.Limiter
	hedgeErrorRateReservation *rate.Reservation

	askPriceHeartBeat, bidPriceHeartBeat types.PriceHeartBeat

	lastPrice fixedpoint.Value

	stopC chan struct{}
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	makerSession, hedgeSession, err := selectSessions2(sessions, s.MakerExchange, s.HedgeExchange)
	if err != nil {
		panic(err)
	}

	hedgeSession.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{})
	hedgeSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
	makerSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
}

func (s *Strategy) Validate() error {
	if s.Quantity.IsZero() || s.QuantityScale == nil {
		return errors.New("quantity or quantityScale can not be empty")
	}

	if len(s.Symbol) == 0 {
		return errors.New("symbol is required")
	}

	return nil
}

func (s *Strategy) Defaults() error {
	if s.UpdateInterval == 0 {
		s.UpdateInterval = types.Duration(time.Second)
	}

	if s.HedgeInterval == 0 {
		s.HedgeInterval = types.Duration(3 * time.Second)
	}

	if s.NumLayers == 0 {
		s.NumLayers = 1
	}

	if s.Margin.IsZero() {
		s.Margin = defaultMargin
	}

	if s.BidMargin.IsZero() {
		if !s.Margin.IsZero() {
			s.BidMargin = s.Margin
		} else {
			s.BidMargin = defaultMargin
		}
	}

	if s.AskMargin.IsZero() {
		if !s.Margin.IsZero() {
			s.AskMargin = s.Margin
		} else {
			s.AskMargin = defaultMargin
		}
	}

	s.hedgeErrorLimiter = rate.NewLimiter(rate.Every(1*time.Minute), 1)
	return nil
}

func (s *Strategy) Initialize() error {
	return nil
}

func (s *Strategy) CrossRun(
	ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession,
) error {
	instanceID := s.InstanceID()

	makerSession, hedgeSession, err := selectSessions2(sessions, s.MakerExchange, s.HedgeExchange)
	if err != nil {
		return err
	}

	s.CrossExchangeMarketMakingStrategy = &CrossExchangeMarketMakingStrategy{}
	if err := s.CrossExchangeMarketMakingStrategy.Initialize(ctx, s.Environment, makerSession, hedgeSession, s.Symbol, ID, s.InstanceID()); err != nil {
		return err
	}

	if s.CoveredPosition.IsZero() {
		if s.state != nil && !s.CoveredPosition.IsZero() {
			s.CoveredPosition = s.state.CoveredPosition
		}
	}

	if s.makerSession.MakerFeeRate.Sign() > 0 || s.makerSession.TakerFeeRate.Sign() > 0 {
		s.Position.SetExchangeFeeRate(types.ExchangeName(s.MakerExchange), types.ExchangeFee{
			MakerFeeRate: s.makerSession.MakerFeeRate,
			TakerFeeRate: s.makerSession.TakerFeeRate,
		})
	}

	if s.hedgeSession.MakerFeeRate.Sign() > 0 || s.hedgeSession.TakerFeeRate.Sign() > 0 {
		s.Position.SetExchangeFeeRate(types.ExchangeName(s.HedgeExchange), types.ExchangeFee{
			MakerFeeRate: s.hedgeSession.MakerFeeRate,
			TakerFeeRate: s.hedgeSession.TakerFeeRate,
		})
	}

	s.pricingBook = types.NewStreamBook(s.Symbol)
	s.pricingBook.BindStream(s.hedgeSession.MarketDataStream)

	s.activeMakerOrders = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeMakerOrders.BindStream(s.makerSession.UserDataStream)

	s.orderStore = core.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(s.hedgeSession.UserDataStream)
	s.orderStore.BindStream(s.makerSession.UserDataStream)
	s.tradeCollector = core.NewTradeCollector(s.Symbol, s.Position, s.orderStore)

	if s.NotifyTrade {
		s.tradeCollector.OnTrade(notifyTrade)
	}

	s.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
		c := trade.PositionChange()
		if trade.Exchange == s.hedgeSession.ExchangeName {
			s.CoveredPosition = s.CoveredPosition.Add(c)
		}

		s.ProfitStats.AddTrade(trade)

		if profit.Compare(fixedpoint.Zero) == 0 {
			s.Environment.RecordPosition(s.Position, trade, nil)
		} else {
			log.Infof("%s generated profit: %v", s.Symbol, profit)

			p := s.Position.NewProfit(trade, profit, netProfit)
			p.Strategy = ID
			p.StrategyInstanceID = instanceID
			bbgo.Notify(&p)
			s.ProfitStats.AddProfit(p)

			s.Environment.RecordPosition(s.Position, trade, &p)
		}
	})

	s.tradeCollector.OnPositionUpdate(func(position *types.Position) {
		bbgo.Notify(position)
	})
	s.tradeCollector.OnRecover(func(trade types.Trade) {
		bbgo.Notify("Recovered trade", trade)
	})
	s.tradeCollector.BindStream(s.hedgeSession.UserDataStream)
	s.tradeCollector.BindStream(s.makerSession.UserDataStream)

	s.stopC = make(chan struct{})

	if s.RecoverTrade {
		go s.tradeRecover(ctx)
	}

	go func() {
		posTicker := time.NewTicker(util.MillisecondsJitter(s.HedgeInterval.Duration(), 200))
		defer posTicker.Stop()

		quoteTicker := time.NewTicker(util.MillisecondsJitter(s.UpdateInterval.Duration(), 200))
		defer quoteTicker.Stop()

		reportTicker := time.NewTicker(time.Hour)
		defer reportTicker.Stop()

		defer func() {
			if err := s.activeMakerOrders.GracefulCancel(context.Background(), s.makerSession.Exchange); err != nil {
				log.WithError(err).Errorf("can not cancel %s orders", s.Symbol)
			}
		}()

		for {
			select {

			case <-s.stopC:
				log.Warnf("%s maker goroutine stopped, due to the stop signal", s.Symbol)
				return

			case <-ctx.Done():
				log.Warnf("%s maker goroutine stopped, due to the cancelled context", s.Symbol)
				return

			case <-quoteTicker.C:
				s.updateQuote(ctx, orderExecutionRouter)

			case <-reportTicker.C:
				bbgo.Notify(s.ProfitStats)

			case <-posTicker.C:
				// For positive position and positive covered position:
				// uncover position = +5 - +3 (covered position) = 2
				//
				// For positive position and negative covered position:
				// uncover position = +5 - (-3) (covered position) = 8
				//
				// meaning we bought 5 on MAX and sent buy order with 3 on binance
				//
				// For negative position:
				// uncover position = -5 - -3 (covered position) = -2
				s.tradeCollector.Process()

				position := s.Position.GetBase()

				uncoverPosition := position.Sub(s.CoveredPosition)
				absPos := uncoverPosition.Abs()
				if !s.DisableHedge && absPos.Compare(s.hedgeMarket.MinQuantity) > 0 {
					log.Infof("%s base position %v coveredPosition: %v uncoverPosition: %v",
						s.Symbol,
						position,
						s.CoveredPosition,
						uncoverPosition,
					)

					s.Hedge(ctx, uncoverPosition.Neg())
				}
			}
		}
	}()

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		close(s.stopC)

		// wait for the quoter to stop
		time.Sleep(s.UpdateInterval.Duration())

		shutdownCtx, cancelShutdown := context.WithTimeout(context.TODO(), time.Minute)
		defer cancelShutdown()

		if err := s.activeMakerOrders.GracefulCancel(shutdownCtx, s.makerSession.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel error")
		}

		bbgo.Notify("%s: %s position", ID, s.Symbol, s.Position)
	})

	return nil
}

func (s *Strategy) Hedge(ctx context.Context, pos fixedpoint.Value) {
	side := types.SideTypeBuy
	if pos.IsZero() {
		return
	}

	quantity := pos.Abs()

	if pos.Sign() < 0 {
		side = types.SideTypeSell
	}

	lastPrice := s.lastPrice
	sourceBook := s.pricingBook.CopyDepth(1)
	switch side {

	case types.SideTypeBuy:
		if bestAsk, ok := sourceBook.BestAsk(); ok {
			lastPrice = bestAsk.Price
		}

	case types.SideTypeSell:
		if bestBid, ok := sourceBook.BestBid(); ok {
			lastPrice = bestBid.Price
		}
	}

	notional := quantity.Mul(lastPrice)
	if notional.Compare(s.hedgeMarket.MinNotional) <= 0 {
		log.Warnf("%s %v less than min notional, skipping hedge", s.Symbol, notional)
		return
	}

	// adjust quantity according to the balances
	account := s.hedgeSession.GetAccount()
	switch side {

	case types.SideTypeBuy:
		// check quote quantity
		if quote, ok := account.Balance(s.hedgeMarket.QuoteCurrency); ok {
			if quote.Available.Compare(notional) < 0 {
				// adjust price to higher 0.1%, so that we can ensure that the order can be executed
				quantity = bbgo.AdjustQuantityByMaxAmount(quantity, lastPrice.Mul(lastPriceModifier), quote.Available)
				quantity = s.hedgeMarket.TruncateQuantity(quantity)
			}
		}

	case types.SideTypeSell:
		// check quote quantity
		if base, ok := account.Balance(s.hedgeMarket.BaseCurrency); ok {
			if base.Available.Compare(quantity) < 0 {
				quantity = base.Available
			}
		}
	}

	// truncate quantity for the supported precision
	quantity = s.hedgeMarket.TruncateQuantity(quantity)

	if notional.Compare(s.hedgeMarket.MinNotional.Mul(minGap)) <= 0 {
		log.Warnf("the adjusted amount %v is less than minimal notional %v, skipping hedge", notional, s.hedgeMarket.MinNotional)
		return
	}

	if quantity.Compare(s.hedgeMarket.MinQuantity.Mul(minGap)) <= 0 {
		log.Warnf("the adjusted quantity %v is less than minimal quantity %v, skipping hedge", quantity, s.hedgeMarket.MinQuantity)
		return
	}

	if s.hedgeErrorRateReservation != nil {
		if !s.hedgeErrorRateReservation.OK() {
			return
		}
		bbgo.Notify("Hit hedge error rate limit, waiting...")
		time.Sleep(s.hedgeErrorRateReservation.Delay())
		s.hedgeErrorRateReservation = nil
	}

	log.Infof("submitting %s hedge order %s %v", s.Symbol, side.String(), quantity)
	bbgo.Notify("Submitting %s hedge order %s %v", s.Symbol, side.String(), quantity)
	orderExecutor := &bbgo.ExchangeOrderExecutor{Session: s.hedgeSession}
	returnOrders, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Market:   s.hedgeMarket,
		Symbol:   s.Symbol,
		Type:     types.OrderTypeMarket,
		Side:     side,
		Quantity: quantity,
	})

	if err != nil {
		s.hedgeErrorRateReservation = s.hedgeErrorLimiter.Reserve()
		log.WithError(err).Errorf("market order submit error: %s", err.Error())
		return
	}

	// if it's selling, than we should add positive position
	if side == types.SideTypeSell {
		s.CoveredPosition = s.CoveredPosition.Add(quantity)
	} else {
		s.CoveredPosition = s.CoveredPosition.Add(quantity.Neg())
	}

	s.orderStore.Add(returnOrders...)
}

func (s *Strategy) tradeRecover(ctx context.Context) {
	tradeScanInterval := s.RecoverTradeScanPeriod.Duration()
	if tradeScanInterval == 0 {
		tradeScanInterval = 30 * time.Minute
	}

	tradeScanOverlapBufferPeriod := 5 * time.Minute

	tradeScanTicker := time.NewTicker(tradeScanInterval)
	defer tradeScanTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-tradeScanTicker.C:
			log.Infof("scanning trades from %s ago...", tradeScanInterval)

			if s.RecoverTrade {
				startTime := time.Now().Add(-tradeScanInterval).Add(-tradeScanOverlapBufferPeriod)

				if err := s.tradeCollector.Recover(ctx, s.hedgeSession.Exchange.(types.ExchangeTradeHistoryService), s.Symbol, startTime); err != nil {
					log.WithError(err).Errorf("query trades error")
				}

				if err := s.tradeCollector.Recover(ctx, s.makerSession.Exchange.(types.ExchangeTradeHistoryService), s.Symbol, startTime); err != nil {
					log.WithError(err).Errorf("query trades error")
				}
			}
		}
	}
}

func (s *Strategy) updateQuote(ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter) {
	if err := s.activeMakerOrders.GracefulCancel(ctx, s.makerSession.Exchange); err != nil {
		log.Warnf("there are some %s orders not canceled, skipping placing maker orders", s.Symbol)
		s.activeMakerOrders.Print()
		return
	}

	if s.activeMakerOrders.NumOfOrders() > 0 {
		return
	}

	bestBid, bestAsk, hasPrice := s.pricingBook.BestBidAndAsk()
	if !hasPrice {
		return
	}

	// use mid-price for the last price
	s.lastPrice = bestBid.Price.Add(bestAsk.Price).Div(Two)

	bookLastUpdateTime := s.pricingBook.LastUpdateTime()

	if _, err := s.bidPriceHeartBeat.Update(bestBid, priceUpdateTimeout); err != nil {
		log.WithError(err).Errorf("quote update error, %s price not updating, order book last update: %s ago",
			s.Symbol,
			time.Since(bookLastUpdateTime))
		return
	}

	if _, err := s.askPriceHeartBeat.Update(bestAsk, priceUpdateTimeout); err != nil {
		log.WithError(err).Errorf("quote update error, %s price not updating, order book last update: %s ago",
			s.Symbol,
			time.Since(bookLastUpdateTime))
		return
	}

	sourceBook := s.pricingBook.CopyDepth(10)
	if valid, err := sourceBook.IsValid(); !valid {
		log.WithError(err).Errorf("%s invalid copied order book, skip quoting: %v", s.Symbol, err)
		return
	}

	var disableMakerBid = false
	var disableMakerAsk = false

	// check maker's balance quota
	// we load the balances from the account while we're generating the orders,
	// the balance may have a chance to be deducted by other strategies or manual orders submitted by the user
	makerBalances := s.makerSession.GetAccount().Balances()
	makerQuota := &bbgo.QuotaTransaction{}
	if b, ok := makerBalances[s.makerMarket.BaseCurrency]; ok {
		if b.Available.Compare(s.makerMarket.MinQuantity) > 0 {
			makerQuota.BaseAsset.Add(b.Available)
		} else {
			disableMakerAsk = true
		}
	}

	if b, ok := makerBalances[s.makerMarket.QuoteCurrency]; ok {
		if b.Available.Compare(s.makerMarket.MinNotional) > 0 {
			makerQuota.QuoteAsset.Add(b.Available)
		} else {
			disableMakerBid = true
		}
	}

	hedgeBalances := s.hedgeSession.GetAccount().Balances()
	hedgeQuota := &bbgo.QuotaTransaction{}
	if b, ok := hedgeBalances[s.hedgeMarket.BaseCurrency]; ok {
		// to make bid orders, we need enough base asset in the foreign exchange,
		// if the base asset balance is not enough for selling
		if s.StopHedgeBaseBalance.Sign() > 0 {
			minAvailable := s.StopHedgeBaseBalance.Add(s.hedgeMarket.MinQuantity)
			if b.Available.Compare(minAvailable) > 0 {
				hedgeQuota.BaseAsset.Add(b.Available.Sub(minAvailable))
			} else {
				log.Warnf("%s maker bid disabled: insufficient base balance %s", s.Symbol, b.String())
				disableMakerBid = true
			}
		} else if b.Available.Compare(s.hedgeMarket.MinQuantity) > 0 {
			hedgeQuota.BaseAsset.Add(b.Available)
		} else {
			log.Warnf("%s maker bid disabled: insufficient base balance %s", s.Symbol, b.String())
			disableMakerBid = true
		}
	}

	if b, ok := hedgeBalances[s.hedgeMarket.QuoteCurrency]; ok {
		// to make ask orders, we need enough quote asset in the foreign exchange,
		// if the quote asset balance is not enough for buying
		if s.StopHedgeQuoteBalance.Sign() > 0 {
			minAvailable := s.StopHedgeQuoteBalance.Add(s.hedgeMarket.MinNotional)
			if b.Available.Compare(minAvailable) > 0 {
				hedgeQuota.QuoteAsset.Add(b.Available.Sub(minAvailable))
			} else {
				log.Warnf("%s maker ask disabled: insufficient quote balance %s", s.Symbol, b.String())
				disableMakerAsk = true
			}
		} else if b.Available.Compare(s.hedgeMarket.MinNotional) > 0 {
			hedgeQuota.QuoteAsset.Add(b.Available)
		} else {
			log.Warnf("%s maker ask disabled: insufficient quote balance %s", s.Symbol, b.String())
			disableMakerAsk = true
		}
	}

	// if max exposure position is configured, we should not:
	// 1. place bid orders when we already bought too much
	// 2. place ask orders when we already sold too much
	if s.MaxExposurePosition.Sign() > 0 {
		pos := s.Position.GetBase()

		if pos.Compare(s.MaxExposurePosition.Neg()) > 0 {
			// stop sell if we over-sell
			disableMakerAsk = true
		} else if pos.Compare(s.MaxExposurePosition) > 0 {
			// stop buy if we over buy
			disableMakerBid = true
		}
	}

	if disableMakerAsk && disableMakerBid {
		log.Warnf("%s bid/ask maker is disabled due to insufficient balances", s.Symbol)
		return
	}

	bestBidPrice := bestBid.Price
	bestAskPrice := bestAsk.Price
	log.Infof("%s book ticker: best ask / best bid = %v / %v", s.Symbol, bestAskPrice, bestBidPrice)

	var submitOrders []types.SubmitOrder
	var accumulativeBidQuantity, accumulativeAskQuantity fixedpoint.Value
	var bidQuantity = s.Quantity
	var askQuantity = s.Quantity
	var bidMargin = s.BidMargin
	var askMargin = s.AskMargin
	var pips = s.Pips

	bidPrice := bestBidPrice
	askPrice := bestAskPrice
	for i := 0; i < s.NumLayers; i++ {
		// for maker bid orders
		if !disableMakerBid {
			if s.QuantityScale != nil {
				qf, err := s.QuantityScale.Scale(i + 1)
				if err != nil {
					log.WithError(err).Errorf("quantityScale error")
					return
				}

				log.Infof("%s scaling bid #%d quantity to %f", s.Symbol, i+1, qf)

				// override the default bid quantity
				bidQuantity = fixedpoint.NewFromFloat(qf)
			}

			accumulativeBidQuantity = accumulativeBidQuantity.Add(bidQuantity)
			if s.UseDepthPrice {
				if s.DepthQuantity.Sign() > 0 {
					bidPrice = aggregatePrice(sourceBook.SideBook(types.SideTypeBuy), s.DepthQuantity)
				} else {
					bidPrice = aggregatePrice(sourceBook.SideBook(types.SideTypeBuy), accumulativeBidQuantity)
				}
			}

			bidPrice = bidPrice.Mul(fixedpoint.One.Sub(bidMargin))
			if i > 0 && pips.Sign() > 0 {
				bidPrice = bidPrice.Sub(pips.Mul(fixedpoint.NewFromInt(int64(i)).
					Mul(s.makerMarket.TickSize)))
			}

			if makerQuota.QuoteAsset.Lock(bidQuantity.Mul(bidPrice)) && hedgeQuota.BaseAsset.Lock(bidQuantity) {
				// if we bought, then we need to sell the base from the hedge session
				submitOrders = append(submitOrders, types.SubmitOrder{
					Symbol:      s.Symbol,
					Type:        types.OrderTypeLimit,
					Side:        types.SideTypeBuy,
					Price:       bidPrice,
					Quantity:    bidQuantity,
					TimeInForce: types.TimeInForceGTC,
				})

				makerQuota.Commit()
				hedgeQuota.Commit()
			} else {
				makerQuota.Rollback()
				hedgeQuota.Rollback()
			}
		}

		// for maker ask orders
		if !disableMakerAsk {
			if s.QuantityScale != nil {
				qf, err := s.QuantityScale.Scale(i + 1)
				if err != nil {
					log.WithError(err).Errorf("quantityScale error")
					return
				}

				log.Infof("%s scaling ask #%d quantity to %f", s.Symbol, i+1, qf)

				// override the default bid quantity
				askQuantity = fixedpoint.NewFromFloat(qf)
			}
			accumulativeAskQuantity = accumulativeAskQuantity.Add(askQuantity)

			if s.UseDepthPrice {
				if s.DepthQuantity.Sign() > 0 {
					askPrice = aggregatePrice(sourceBook.SideBook(types.SideTypeSell), s.DepthQuantity)
				} else {
					askPrice = aggregatePrice(sourceBook.SideBook(types.SideTypeSell), accumulativeAskQuantity)
				}
			}

			askPrice = askPrice.Mul(fixedpoint.One.Add(askMargin))
			if i > 0 && pips.Sign() > 0 {
				askPrice = askPrice.Add(pips.Mul(fixedpoint.NewFromInt(int64(i)).Mul(s.makerMarket.TickSize)))
			}

			if makerQuota.BaseAsset.Lock(askQuantity) && hedgeQuota.QuoteAsset.Lock(askQuantity.Mul(askPrice)) {
				// if we bought, then we need to sell the base from the hedge session
				submitOrders = append(submitOrders, types.SubmitOrder{
					Symbol:      s.Symbol,
					Market:      s.makerMarket,
					Type:        types.OrderTypeLimit,
					Side:        types.SideTypeSell,
					Price:       askPrice,
					Quantity:    askQuantity,
					TimeInForce: types.TimeInForceGTC,
				})
				makerQuota.Commit()
				hedgeQuota.Commit()
			} else {
				makerQuota.Rollback()
				hedgeQuota.Rollback()
			}

		}
	}

	if len(submitOrders) == 0 {
		log.Warnf("no orders generated")
		return
	}

	makerOrders, err := orderExecutionRouter.SubmitOrdersTo(ctx, s.MakerExchange, submitOrders...)
	if err != nil {
		log.WithError(err).Errorf("order error: %s", err.Error())
		return
	}

	s.activeMakerOrders.Add(makerOrders...)
	s.orderStore.Add(makerOrders...)
}

func selectSessions2(
	sessions map[string]*bbgo.ExchangeSession, n1, n2 string,
) (s1, s2 *bbgo.ExchangeSession, err error) {
	for _, n := range []string{n1, n2} {
		if _, ok := sessions[n]; !ok {
			return nil, nil, fmt.Errorf("session %s is not defined", n)
		}
	}

	s1 = sessions[n1]
	s2 = sessions[n2]
	return s1, s2, nil
}

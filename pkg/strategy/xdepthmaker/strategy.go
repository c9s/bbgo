package xdepthmaker

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/pricesolver"
	"github.com/c9s/bbgo/pkg/sigchan"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/strategy/xmaker"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/c9s/bbgo/pkg/util/tradingutil"
)

var lastPriceModifier = fixedpoint.NewFromFloat(1.001)
var minGap = fixedpoint.NewFromFloat(1.02)
var defaultMargin = fixedpoint.NewFromFloat(0.003)

var Two = fixedpoint.NewFromInt(2)

const priceUpdateTimeout = 5 * time.Minute

const ID = "xdepthmaker"

var log = logrus.WithField("strategy", ID)

var ErrZeroQuantity = stderrors.New("quantity is zero")
var ErrDustQuantity = stderrors.New("quantity is dust")
var ErrZeroPrice = stderrors.New("price is zero")

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type CrossExchangeMarketMakingStrategy struct {
	ctx, parent context.Context
	cancel      context.CancelFunc

	Environ *bbgo.Environment

	makerSession, hedgeSession *bbgo.ExchangeSession
	makerMarket, hedgeMarket   types.Market

	// persistence fields
	Position    *types.Position    `json:"position,omitempty" persistence:"position"`
	ProfitStats *types.ProfitStats `json:"profitStats,omitempty" persistence:"profit_stats"`

	CoveredPosition fixedpoint.MutexValue

	core.ConverterManager

	mu sync.Mutex

	MakerOrderExecutor, HedgeOrderExecutor *bbgo.GeneralOrderExecutor
}

func (s *CrossExchangeMarketMakingStrategy) Initialize(
	ctx context.Context, environ *bbgo.Environment,
	makerSession, hedgeSession *bbgo.ExchangeSession,
	symbol, hedgeSymbol,
	strategyID, instanceID string,
) error {
	s.parent = ctx
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.Environ = environ

	s.makerSession = makerSession
	s.hedgeSession = hedgeSession

	var ok bool
	s.hedgeMarket, ok = s.hedgeSession.Market(hedgeSymbol)
	if !ok {
		return fmt.Errorf("hedge session market %s is not defined", hedgeSymbol)
	}

	s.makerMarket, ok = s.makerSession.Market(symbol)
	if !ok {
		return fmt.Errorf("maker session market %s is not defined", symbol)
	}

	if err := s.ConverterManager.Initialize(); err != nil {
		return err
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

	// update converter manager
	s.MakerOrderExecutor.TradeCollector().ConverterManager = s.ConverterManager

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

	s.HedgeOrderExecutor.TradeCollector().ConverterManager = s.ConverterManager

	s.HedgeOrderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		// bbgo.Sync(ctx, s)
	})

	s.HedgeOrderExecutor.ActiveMakerOrders().OnCanceled(func(o types.Order) {
		remaining := o.Quantity.Sub(o.ExecutedQuantity)

		log.Infof("canceled order #%d, remaining quantity: %f", o.OrderID, remaining.Float64())

		switch o.Side {
		case types.SideTypeSell:
			remaining = remaining.Neg()
		}

		remaining = remaining.Neg()
		coveredPosition := s.CoveredPosition.Get()
		s.CoveredPosition.Sub(remaining)

		log.Infof("coveredPosition %f - %f => %f", coveredPosition.Float64(), remaining.Float64(), s.CoveredPosition.Get().Float64())
	})

	s.HedgeOrderExecutor.TradeCollector().OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
		c := trade.PositionChange()

		// sync covered position
		// sell trade -> negative delta ->
		// 	  1) long position -> reduce long position
		//    2) short position -> increase short position
		// buy trade -> positive delta ->
		// 	  1) short position -> reduce short position
		// 	  2) short position -> increase short position
		s.CoveredPosition.Add(c)
	})
	return nil
}

type HedgeStrategy string

const (
	HedgeStrategyMarket           HedgeStrategy = "market"
	HedgeStrategyBboCounterParty1 HedgeStrategy = "bbo-counter-party-1"
	HedgeStrategyBboCounterParty3 HedgeStrategy = "bbo-counter-party-3"
	HedgeStrategyBboCounterParty5 HedgeStrategy = "bbo-counter-party-5"
	HedgeStrategyBboQueue1        HedgeStrategy = "bbo-queue-1"
)

type Strategy struct {
	*CrossExchangeMarketMakingStrategy

	Environment *bbgo.Environment

	// Symbol is the maker exchange symbol
	Symbol string `json:"symbol"`

	// HedgeSymbol is the symbol for the hedge exchange
	// symbol could be different from the maker exchange
	HedgeSymbol string `json:"hedgeSymbol"`

	// MakerExchange session name
	MakerExchange string `json:"makerExchange"`

	// HedgeExchange session name
	HedgeExchange string `json:"hedgeExchange"`

	FastLayerUpdateInterval types.Duration `json:"fastLayerUpdateInterval"`
	NumOfFastLayers         int            `json:"numOfFastLayers"`

	HedgeInterval types.Duration `json:"hedgeInterval"`

	HedgeStrategy HedgeStrategy `json:"hedgeStrategy"`

	HedgeMaxOrderQuantity fixedpoint.Value `json:"hedgeMaxOrderQuantity"`

	FullReplenishInterval types.Duration `json:"fullReplenishInterval"`

	OrderCancelWaitTime types.Duration `json:"orderCancelWaitTime"`

	Margin    fixedpoint.Value `json:"margin"`
	BidMargin fixedpoint.Value `json:"bidMargin"`
	AskMargin fixedpoint.Value `json:"askMargin"`

	StopHedgeQuoteBalance fixedpoint.Value `json:"stopHedgeQuoteBalance"`
	StopHedgeBaseBalance  fixedpoint.Value `json:"stopHedgeBaseBalance"`

	// Quantity is used for fixed quantity of the first layer
	Quantity fixedpoint.Value `json:"quantity"`

	// QuantityScale helps user to define the quantity by layer scale
	QuantityScale *bbgo.LayerScale `json:"quantityScale,omitempty"`

	// DepthScale helps user to define the depth by layer scale
	DepthScale *bbgo.LayerScale `json:"depthScale,omitempty"`

	// MaxExposurePosition defines the unhedged quantity of stop
	MaxExposurePosition fixedpoint.Value `json:"maxExposurePosition"`

	DisableHedge bool `json:"disableHedge"`

	NotifyTrade bool `json:"notifyTrade"`

	// RecoverTrade tries to find the missing trades via the REStful API
	RecoverTrade bool `json:"recoverTrade"`

	PriceImpactRatio fixedpoint.Value `json:"priceImpactRatio"`

	RecoverTradeScanPeriod types.Duration `json:"recoverTradeScanPeriod"`

	NumLayers int `json:"numLayers"`

	// Pips is the pips of the layer prices
	Pips fixedpoint.Value `json:"pips"`

	ProfitFixerConfig *common.ProfitFixerConfig `json:"profitFixer,omitempty"`

	// --------------------------------
	// private fields
	// --------------------------------

	// pricingBook is the order book (depth) from the hedging session
	sourceBook *types.StreamOrderBook

	hedgeErrorLimiter         *rate.Limiter
	hedgeErrorRateReservation *rate.Reservation

	askPriceHeartBeat, bidPriceHeartBeat *types.PriceHeartBeat

	lastSourcePrice fixedpoint.MutexValue

	stopC                 chan struct{}
	fullReplenishTriggerC sigchan.Chan

	logger logrus.FieldLogger

	makerConnectivity, hedgerConnectivity *types.Connectivity
	connectivityGroup                     *types.ConnectivityGroup

	priceSolver *pricesolver.SimplePriceSolver
	bboMonitor  *bbgo.BboMonitor
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	// this generates a unique instance ID for the strategy
	return strings.Join([]string{
		ID,
		s.MakerExchange,
		s.Symbol,
		s.HedgeExchange,
		s.HedgeSymbol,
	}, "-")
}

func (s *Strategy) Initialize() error {
	if s.CrossExchangeMarketMakingStrategy == nil {
		s.CrossExchangeMarketMakingStrategy = &CrossExchangeMarketMakingStrategy{}
	}

	s.bidPriceHeartBeat = types.NewPriceHeartBeat(priceUpdateTimeout)
	s.askPriceHeartBeat = types.NewPriceHeartBeat(priceUpdateTimeout)
	s.logger = log.WithFields(logrus.Fields{
		"symbol":            s.Symbol,
		"strategy":          ID,
		"strategy_instance": s.InstanceID(),
	})

	return nil
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	makerSession, hedgeSession, err := selectSessions2(sessions, s.MakerExchange, s.HedgeExchange)
	if err != nil {
		panic(err)
	}

	hedgeSession.Subscribe(types.BookChannel, s.HedgeSymbol, types.SubscribeOptions{
		Depth: types.DepthLevelMedium,
		Speed: types.SpeedLow,
	})

	hedgeSession.Subscribe(types.KLineChannel, s.HedgeSymbol, types.SubscribeOptions{Interval: "1m"})
	hedgeSession.Subscribe(types.KLineChannel, hedgeSession.Exchange.PlatformFeeCurrency()+"USDT", types.SubscribeOptions{Interval: "1m"})

	makerSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
	makerSession.Subscribe(types.KLineChannel, makerSession.Exchange.PlatformFeeCurrency()+"USDT", types.SubscribeOptions{Interval: "1m"})
}

func (s *Strategy) Validate() error {
	if s.MakerExchange == "" {
		return errors.New("maker exchange is not configured")
	}

	if s.HedgeExchange == "" {
		return errors.New("hedge exchange is not configured")
	}

	if s.DepthScale == nil {
		return errors.New("depthScale can not be empty")
	}

	if len(s.Symbol) == 0 {
		return errors.New("symbol is required")
	}

	return nil
}

func (s *Strategy) Defaults() error {
	if s.FastLayerUpdateInterval == 0 {
		s.FastLayerUpdateInterval = types.Duration(5 * time.Second)
	}

	if s.NumOfFastLayers == 0 {
		s.NumOfFastLayers = 5
	}

	if s.FullReplenishInterval == 0 {
		s.FullReplenishInterval = types.Duration(10 * time.Minute)
	}

	if s.HedgeInterval == 0 {
		s.HedgeInterval = types.Duration(3 * time.Second)
	}

	if s.HedgeStrategy == "" {
		s.HedgeStrategy = HedgeStrategyMarket
	}

	if s.HedgeSymbol == "" {
		s.HedgeSymbol = s.Symbol
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

func (s *Strategy) quoteWorker(ctx context.Context) {
	updateTicker := time.NewTicker(util.MillisecondsJitter(s.FastLayerUpdateInterval.Duration(), 200))
	defer updateTicker.Stop()

	fullReplenishTicker := time.NewTicker(util.MillisecondsJitter(s.FullReplenishInterval.Duration(), 200))
	defer fullReplenishTicker.Stop()

	// clean up the previous open orders
	if err := s.cleanUpOpenOrders(ctx, s.makerSession); err != nil {
		log.WithError(err).Warnf("error cleaning up open orders")
	}

	s.updateQuote(ctx, 0)

	for {
		select {
		case <-ctx.Done():
			return

		case <-s.stopC:
			log.Warnf("%s maker goroutine stopped, due to the stop signal", s.Symbol)
			return

		case <-s.fullReplenishTriggerC:
			// force trigger full replenish
			s.updateQuote(ctx, 0)

		case <-fullReplenishTicker.C:
			s.updateQuote(ctx, 0)

		case <-updateTicker.C:
			s.updateQuote(ctx, s.NumOfFastLayers)

		case sig, ok := <-s.sourceBook.C:
			// when any book change event happened
			if !ok {
				return
			}

			changed := s.bboMonitor.UpdateFromBook(s.sourceBook)
			if changed || sig.Type == types.BookSignalSnapshot {
				s.updateQuote(ctx, 0)
			}
		}
	}
}

func (s *Strategy) hedgeWorker(ctx context.Context) {
	ticker := time.NewTicker(util.MillisecondsJitter(s.HedgeInterval.Duration(), 200))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Warnf("maker goroutine stopped, due to context canceled")
			return

		case <-s.stopC:
			s.logger.Warnf("maker goroutine stopped, due to the stop signal")
			return

		case <-ticker.C:
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
			s.HedgeOrderExecutor.TradeCollector().Process()
			s.MakerOrderExecutor.TradeCollector().Process()

			position := s.Position.GetBase()

			coveredPosition := s.CoveredPosition.Get()
			uncoverPosition := position.Sub(coveredPosition)

			absPos := uncoverPosition.Abs()
			if !s.hedgeMarket.IsDustQuantity(absPos, s.lastSourcePrice.Get()) {
				log.Infof("%s base position %v coveredPosition: %v uncoverPosition: %v",
					s.Symbol,
					position,
					coveredPosition,
					uncoverPosition,
				)

				if !s.DisableHedge {
					if err := s.Hedge(ctx, uncoverPosition.Neg()); err != nil {
						//goland:noinspection GoDirectComparisonOfErrors
						switch err {
						case ErrZeroQuantity, ErrDustQuantity:
						default:
							s.logger.WithError(err).Errorf("unable to hedge position")
						}
					}
				}
			}
		}
	}
}

func (s *Strategy) CrossRun(
	ctx context.Context, _ bbgo.OrderExecutionRouter,
	sessions map[string]*bbgo.ExchangeSession,
) error {
	makerSession, hedgeSession, err := selectSessions2(sessions, s.MakerExchange, s.HedgeExchange)
	if err != nil {
		return err
	}

	log.Infof("makerSession: %s hedgeSession: %s", makerSession.Name, hedgeSession.Name)

	if s.ProfitFixerConfig != nil {
		bbgo.Notify("Fixing %s profitStats and position...", s.Symbol)

		log.Infof("profitFixer is enabled, checking checkpoint: %+v", s.ProfitFixerConfig.TradesSince)

		if s.ProfitFixerConfig.TradesSince.Time().IsZero() {
			return errors.New("tradesSince time can not be zero")
		}

		makerMarket, _ := makerSession.Market(s.Symbol)
		s.CrossExchangeMarketMakingStrategy.Position = types.NewPositionFromMarket(makerMarket)
		s.CrossExchangeMarketMakingStrategy.ProfitStats = types.NewProfitStats(makerMarket)

		fixer := common.NewProfitFixer()
		fixer.ConverterManager = s.ConverterManager

		if ss, ok := makerSession.Exchange.(types.ExchangeTradeHistoryService); ok {
			log.Infof("adding makerSession %s to profitFixer", makerSession.Name)
			fixer.AddExchange(makerSession.Name, ss)
		}

		if ss, ok := hedgeSession.Exchange.(types.ExchangeTradeHistoryService); ok {
			log.Infof("adding hedgeSession %s to profitFixer", hedgeSession.Name)
			fixer.AddExchange(hedgeSession.Name, ss)
		}

		if err2 := fixer.Fix(ctx, makerMarket.Symbol,
			s.ProfitFixerConfig.TradesSince.Time(),
			time.Now(),
			s.CrossExchangeMarketMakingStrategy.ProfitStats,
			s.CrossExchangeMarketMakingStrategy.Position); err2 != nil {
			return err2
		}

		bbgo.Notify("Fixed %s position", s.Symbol, s.CrossExchangeMarketMakingStrategy.Position)
		bbgo.Notify("Fixed %s profitStats", s.Symbol, s.CrossExchangeMarketMakingStrategy.ProfitStats)
	}

	if err := s.CrossExchangeMarketMakingStrategy.Initialize(ctx,
		s.Environment,
		makerSession, hedgeSession,
		s.Symbol, s.HedgeSymbol,
		ID, s.InstanceID()); err != nil {
		return err
	}

	s.sourceBook = types.NewStreamBook(s.HedgeSymbol, s.hedgeSession.ExchangeName)
	s.sourceBook.BindStream(s.hedgeSession.MarketDataStream)

	s.priceSolver = pricesolver.NewSimplePriceResolver(s.makerSession.Markets())
	s.priceSolver.BindStream(s.hedgeSession.MarketDataStream)
	s.priceSolver.BindStream(s.makerSession.MarketDataStream)

	s.bboMonitor = bbgo.NewBboMonitor()
	if !s.PriceImpactRatio.IsZero() {
		s.bboMonitor.SetPriceImpactRatio(s.PriceImpactRatio)
	}

	if err := s.priceSolver.UpdateFromTickers(ctx, s.makerSession.Exchange,
		s.Symbol, s.makerSession.Exchange.PlatformFeeCurrency()+"USDT"); err != nil {
		return err
	}

	if err := s.priceSolver.UpdateFromTickers(ctx, s.hedgeSession.Exchange, s.HedgeSymbol); err != nil {
		return err
	}

	s.makerSession.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval1m, func(k types.KLine) {
		s.priceSolver.Update(k.Symbol, k.Close)
		feeToken := s.makerSession.Exchange.PlatformFeeCurrency()
		if feePrice, ok := s.priceSolver.ResolvePrice(feeToken, "USDT"); ok {
			s.Position.SetFeeAverageCost(feeToken, feePrice)
		}
	}))

	s.stopC = make(chan struct{})
	s.fullReplenishTriggerC = sigchan.New(1)

	s.makerConnectivity = types.NewConnectivity()
	s.makerConnectivity.Bind(s.makerSession.UserDataStream)

	s.hedgerConnectivity = types.NewConnectivity()
	s.hedgerConnectivity.Bind(s.hedgeSession.UserDataStream)

	connGroup := types.NewConnectivityGroup(s.makerConnectivity, s.hedgerConnectivity)
	s.connectivityGroup = connGroup

	if s.RecoverTrade {
		go s.runTradeRecover(ctx)
	}

	go func() {
		log.Infof("waiting for user data stream to get authenticated")

		select {
		case <-ctx.Done():
			return
		case <-connGroup.AllAuthedC(ctx, time.Minute):
		}

		log.Infof("user data stream authenticated, start placing orders...")

		go s.hedgeWorker(ctx)
		go s.quoteWorker(ctx)
	}()

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		bbgo.Notify("Shutting down %s: %s", ID, s.Symbol)

		close(s.stopC)

		// wait for the quoter to stop
		time.Sleep(s.FastLayerUpdateInterval.Duration())

		if err := s.MakerOrderExecutor.GracefulCancel(ctx); err != nil {
			log.WithError(err).Errorf("graceful cancel %s order error", s.Symbol)
		}

		if err := s.HedgeOrderExecutor.GracefulCancel(ctx); err != nil {
			log.WithError(err).Errorf("graceful cancel %s order error", s.HedgeSymbol)
		}

		if err := tradingutil.UniversalCancelAllOrders(ctx, s.makerSession.Exchange, s.Symbol, s.MakerOrderExecutor.ActiveMakerOrders().Orders()); err != nil {
			log.WithError(err).Errorf("unable to cancel all orders")
		}

		// process collected trades
		s.HedgeOrderExecutor.TradeCollector().Process()
		s.MakerOrderExecutor.TradeCollector().Process()

		bbgo.Sync(ctx, s)

		bbgo.Notify("Shutdown %s: %s position", ID, s.Symbol, s.Position)
	})

	return nil
}

func (s *Strategy) Hedge(ctx context.Context, pos fixedpoint.Value) error {
	if pos.IsZero() {
		return nil
	}

	// the default side
	side := types.SideTypeBuy
	if pos.Sign() < 0 {
		side = types.SideTypeSell
	}

	quantity := pos.Abs()

	if s.HedgeMaxOrderQuantity.Sign() > 0 && quantity.Compare(s.HedgeMaxOrderQuantity) > 0 {
		s.logger.Infof("hedgeMaxOrderQuantity is set to %s, limiting the given quantity %s", s.HedgeMaxOrderQuantity.String(), quantity.String())
		quantity = fixedpoint.Min(s.HedgeMaxOrderQuantity, quantity)
	}

	defer func() {
		s.fullReplenishTriggerC.Emit()
	}()

	switch s.HedgeStrategy {
	case HedgeStrategyMarket:
		return s.executeHedgeMarket(ctx, side, quantity)
	case HedgeStrategyBboCounterParty1:
		return s.executeHedgeBboCounterPartyWithIndex(ctx, side, 1, quantity)
	case HedgeStrategyBboCounterParty3:
		return s.executeHedgeBboCounterPartyWithIndex(ctx, side, 3, quantity)
	case HedgeStrategyBboCounterParty5:
		return s.executeHedgeBboCounterPartyWithIndex(ctx, side, 5, quantity)
	case HedgeStrategyBboQueue1:
		return s.executeHedgeBboQueue1(ctx, side, quantity)
	default:
		return fmt.Errorf("unsupported or invalid hedge strategy setup %q, please check your configuration", s.HedgeStrategy)
	}
}

func (s *Strategy) executeHedgeBboCounterPartyWithIndex(
	ctx context.Context,
	side types.SideType,
	idx int,
	quantity fixedpoint.Value,
) error {
	price := s.lastSourcePrice.Get()

	sideBook := s.sourceBook.SideBook(side.Reverse())
	if pv, ok := sideBook.ElemOrLast(idx); ok {
		price = pv.Price
	}

	if price.IsZero() {
		return ErrZeroPrice
	}

	// adjust quantity according to the balances
	account := s.hedgeSession.GetAccount()

	quantity = xmaker.AdjustHedgeQuantityWithAvailableBalance(account,
		s.hedgeMarket,
		side,
		quantity,
		price)

	// truncate quantity for the supported precision
	quantity = s.hedgeMarket.TruncateQuantity(quantity)
	if quantity.IsZero() {
		return ErrZeroQuantity
	}

	if s.hedgeMarket.IsDustQuantity(quantity, price) {
		return ErrDustQuantity
	}

	// submit order as limit taker
	return s.executeHedgeOrder(ctx, types.SubmitOrder{
		Market:   s.hedgeMarket,
		Symbol:   s.hedgeMarket.Symbol,
		Type:     types.OrderTypeLimit,
		Price:    price,
		Side:     side,
		Quantity: quantity,
	})
}

func (s *Strategy) executeHedgeBboQueue1(
	ctx context.Context,
	side types.SideType,
	quantity fixedpoint.Value,
) error {
	price := s.lastSourcePrice.Get()
	if sourcePrice := s.getSourceBboPrice(side); sourcePrice.Sign() > 0 {
		price = sourcePrice
	}

	if price.IsZero() {
		return ErrZeroPrice
	}

	// adjust quantity according to the balances
	account := s.hedgeSession.GetAccount()

	quantity = xmaker.AdjustHedgeQuantityWithAvailableBalance(account,
		s.hedgeMarket,
		side,
		quantity,
		price)

	// truncate quantity for the supported precision
	quantity = s.hedgeMarket.TruncateQuantity(quantity)
	if quantity.IsZero() {
		return ErrZeroQuantity
	}

	if s.hedgeMarket.IsDustQuantity(quantity, price) {
		return ErrDustQuantity
	}

	// submit order as limit taker
	return s.executeHedgeOrder(ctx, types.SubmitOrder{
		Market:   s.hedgeMarket,
		Symbol:   s.hedgeMarket.Symbol,
		Type:     types.OrderTypeLimit,
		Price:    price,
		Side:     side,
		Quantity: quantity,
	})
}

func (s *Strategy) executeHedgeMarket(
	ctx context.Context,
	side types.SideType,
	quantity fixedpoint.Value,
) error {
	price := s.lastSourcePrice.Get()
	if sourcePrice := s.getSourceBboPrice(side.Reverse()); sourcePrice.Sign() > 0 {
		price = sourcePrice
	}

	if price.IsZero() {
		return ErrZeroPrice
	}

	// adjust quantity according to the balances
	account := s.hedgeSession.GetAccount()

	quantity = xmaker.AdjustHedgeQuantityWithAvailableBalance(account,
		s.hedgeMarket,
		side,
		quantity,
		price)

	// truncate quantity for the supported precision
	quantity = s.hedgeMarket.TruncateQuantity(quantity)
	if quantity.IsZero() {
		return ErrZeroQuantity
	}

	if s.hedgeMarket.IsDustQuantity(quantity, price) {
		return ErrDustQuantity
	}

	return s.executeHedgeOrder(ctx, types.SubmitOrder{
		Market:   s.hedgeMarket,
		Symbol:   s.hedgeMarket.Symbol,
		Type:     types.OrderTypeMarket,
		Side:     side,
		Quantity: quantity,
	})
}

// getSourceBboPrice returns the best bid offering price from the source order book
func (s *Strategy) getSourceBboPrice(side types.SideType) fixedpoint.Value {
	bid, ask, ok := s.sourceBook.BestBidAndAsk()
	if !ok {
		return fixedpoint.Zero
	}

	switch side {
	case types.SideTypeSell:
		return ask.Price
	case types.SideTypeBuy:
		return bid.Price
	}
	return fixedpoint.Zero
}

func (s *Strategy) executeHedgeOrder(ctx context.Context, submitOrder types.SubmitOrder) error {
	if err := s.HedgeOrderExecutor.GracefulCancel(ctx); err != nil {
		s.logger.WithError(err).Warnf("graceful cancel order error")
	}

	if s.hedgeErrorRateReservation != nil {
		if !s.hedgeErrorRateReservation.OK() {
			s.logger.Warnf("rate reservation hitted, skip executing hedge order")
			return nil
		}

		bbgo.Notify("Hit hedge error rate limit, waiting...")
		time.Sleep(s.hedgeErrorRateReservation.Delay())

		// reset reservation
		s.hedgeErrorRateReservation = nil
	}

	bbgo.Notify("Submitting hedge %s order on %s %s %s %s @ %s",
		submitOrder.Type, s.HedgeSymbol, s.HedgeExchange,
		submitOrder.Side.String(),
		submitOrder.Quantity.String(),
		submitOrder.Price.String(),
	)

	_, err := s.HedgeOrderExecutor.SubmitOrders(ctx, submitOrder)
	if err != nil {
		// allocate a new reservation
		s.hedgeErrorRateReservation = s.hedgeErrorLimiter.Reserve()
		return err
	}

	// if the hedge is on sell side, then we should add positive position
	switch submitOrder.Side {
	case types.SideTypeSell:
		s.CoveredPosition.Add(submitOrder.Quantity)
	case types.SideTypeBuy:
		s.CoveredPosition.Add(submitOrder.Quantity.Neg())
	}

	return nil
}

func (s *Strategy) runTradeRecover(ctx context.Context) {
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

			startTime := time.Now().Add(-tradeScanInterval).Add(-tradeScanOverlapBufferPeriod)

			if err := s.HedgeOrderExecutor.TradeCollector().Recover(ctx, s.hedgeSession.Exchange.(types.ExchangeTradeHistoryService), s.HedgeSymbol, startTime); err != nil {
				log.WithError(err).Errorf("query trades error")
			}

			if err := s.MakerOrderExecutor.TradeCollector().Recover(ctx, s.makerSession.Exchange.(types.ExchangeTradeHistoryService), s.Symbol, startTime); err != nil {
				log.WithError(err).Errorf("query trades error")
			}
		}
	}
}

func (s *Strategy) generateMakerOrders(
	pricingBook *types.StreamOrderBook,
	maxLayer int,
	availableBase, availableQuote fixedpoint.Value,
) ([]types.SubmitOrder, error) {
	_, _, hasPrice := pricingBook.BestBidAndAsk()
	if !hasPrice {
		return nil, nil
	}

	var submitOrders []types.SubmitOrder
	var accumulatedBidQuantity = fixedpoint.Zero
	var accumulatedAskQuantity = fixedpoint.Zero
	var accumulatedBidQuoteQuantity = fixedpoint.Zero

	// copy the pricing book because during the generation the book data could change
	dupPricingBook := pricingBook.Copy()

	log.Infof("pricingBook: \n\tbids: %+v \n\tasks: %+v",
		dupPricingBook.SideBook(types.SideTypeBuy),
		dupPricingBook.SideBook(types.SideTypeSell))

	if maxLayer == 0 || maxLayer > s.NumLayers {
		maxLayer = s.NumLayers
	}

	var availableBalances = map[types.SideType]fixedpoint.Value{
		types.SideTypeBuy:  availableQuote,
		types.SideTypeSell: availableBase,
	}

	for _, side := range []types.SideType{types.SideTypeBuy, types.SideTypeSell} {
		sideBook := dupPricingBook.SideBook(side)
		if sideBook.Len() == 0 {
			log.Warnf("orderbook %s side is empty", side)
			continue
		}

		availableSideBalance, ok := availableBalances[side]
		if !ok {
			log.Warnf("no available balance for side %s side", side)
			continue
		}

		accumulatedDepth := fixedpoint.Zero
		lastMakerPrice := fixedpoint.Zero

	layerLoop:
		for i := 1; i <= maxLayer; i++ {
			// simple break, we need to check the market minNotional and minQuantity later
			if !availableSideBalance.Eq(fixedpoint.PosInf) {
				if availableSideBalance.IsZero() || availableSideBalance.Sign() < 0 {
					break layerLoop
				}
			}

			requiredDepthFloat, err := s.DepthScale.Scale(i)
			if err != nil {
				return nil, errors.Wrapf(err, "depthScale scale error")
			}

			// requiredDepth is the required depth in quote currency
			requiredDepth := fixedpoint.NewFromFloat(requiredDepthFloat)
			accumulatedDepth = accumulatedDepth.Add(requiredDepth)

			index := sideBook.IndexByQuoteVolumeDepth(accumulatedDepth)

			pvs := types.PriceVolumeSlice{}
			if index == -1 {
				pvs = sideBook[:]
			} else {
				pvs = sideBook[0 : index+1]
			}

			if len(pvs) == 0 {
				continue
			}

			depthPrice := pvs.AverageDepthPriceByQuote(accumulatedDepth, 0)

			switch side {
			case types.SideTypeBuy:
				if s.BidMargin.Sign() > 0 {
					depthPrice = depthPrice.Mul(fixedpoint.One.Sub(s.BidMargin))
				}

				depthPrice = depthPrice.Round(s.makerMarket.PricePrecision+1, fixedpoint.Down)

			case types.SideTypeSell:
				if s.AskMargin.Sign() > 0 {
					depthPrice = depthPrice.Mul(fixedpoint.One.Add(s.AskMargin))
				}

				depthPrice = depthPrice.Round(s.makerMarket.PricePrecision+1, fixedpoint.Up)
			}

			depthPrice = s.makerMarket.TruncatePrice(depthPrice)

			if lastMakerPrice.Sign() > 0 && depthPrice.Compare(lastMakerPrice) == 0 {
				switch side {
				case types.SideTypeBuy:
					depthPrice = depthPrice.Sub(s.makerMarket.TickSize)
				case types.SideTypeSell:
					depthPrice = depthPrice.Add(s.makerMarket.TickSize)
				}
			}

			quantity := requiredDepth.Div(depthPrice)
			quantity = s.makerMarket.TruncateQuantity(quantity)

			s.logger.Infof("%d) %s required depth: %f %s@%s", i, side, accumulatedDepth.Float64(), quantity.String(), depthPrice.String())

			switch side {
			case types.SideTypeBuy:
				quantity = quantity.Sub(accumulatedBidQuantity)

				accumulatedBidQuantity = accumulatedBidQuantity.Add(quantity)
				quoteQuantity := fixedpoint.Mul(quantity, depthPrice)
				quoteQuantity = quoteQuantity.Round(s.makerMarket.PricePrecision, fixedpoint.Up)

				if !availableSideBalance.Eq(fixedpoint.PosInf) && availableSideBalance.Compare(quoteQuantity) <= 0 {
					quoteQuantity = availableSideBalance
					quantity = quoteQuantity.Div(depthPrice).Round(s.makerMarket.PricePrecision, fixedpoint.Down)
				}

				if quantity.Compare(s.makerMarket.MinQuantity) <= 0 || quoteQuantity.Compare(s.makerMarket.MinNotional) <= 0 {
					break layerLoop
				}

				availableSideBalance = availableSideBalance.Sub(quoteQuantity)

				accumulatedBidQuoteQuantity = accumulatedBidQuoteQuantity.Add(quoteQuantity)

			case types.SideTypeSell:
				quantity = quantity.Sub(accumulatedAskQuantity)
				quoteQuantity := quantity.Mul(depthPrice)

				// balance check
				if !availableSideBalance.Eq(fixedpoint.PosInf) && availableSideBalance.Compare(quantity) <= 0 {
					break layerLoop
				}

				if quantity.Compare(s.makerMarket.MinQuantity) <= 0 || quoteQuantity.Compare(s.makerMarket.MinNotional) <= 0 {
					break layerLoop
				}

				availableSideBalance = availableSideBalance.Sub(quantity)

				accumulatedAskQuantity = accumulatedAskQuantity.Add(quantity)
			}

			submitOrders = append(submitOrders, types.SubmitOrder{
				Symbol:   s.makerMarket.Symbol,
				Type:     types.OrderTypeLimitMaker,
				Market:   s.makerMarket,
				Side:     side,
				Price:    depthPrice,
				Quantity: quantity,
			})

			lastMakerPrice = depthPrice
		}
	}

	return submitOrders, nil
}

func (s *Strategy) partiallyCancelOrders(ctx context.Context, maxLayer int) error {
	buyOrders, sellOrders := s.MakerOrderExecutor.ActiveMakerOrders().Orders().SeparateBySide()
	buyOrders = types.SortOrdersByPrice(buyOrders, true)
	sellOrders = types.SortOrdersByPrice(sellOrders, false)

	buyOrdersToCancel := buyOrders[0:min(maxLayer, len(buyOrders))]
	sellOrdersToCancel := sellOrders[0:min(maxLayer, len(sellOrders))]

	err1 := s.MakerOrderExecutor.GracefulCancel(ctx, buyOrdersToCancel...)
	err2 := s.MakerOrderExecutor.GracefulCancel(ctx, sellOrdersToCancel...)
	return stderrors.Join(err1, err2)
}

func (s *Strategy) updateQuote(ctx context.Context, maxLayer int) {
	if maxLayer == 0 {
		if err := s.MakerOrderExecutor.GracefulCancel(ctx); err != nil {
			log.WithError(err).Warnf("there are some %s orders not canceled, skipping placing maker orders", s.Symbol)
			s.MakerOrderExecutor.ActiveMakerOrders().Print()
			return
		}
	} else {
		if err := s.partiallyCancelOrders(ctx, maxLayer); err != nil {
			log.WithError(err).Warnf("%s partial order cancel failed", s.Symbol)
			return
		}
	}

	numOfMakerOrders := s.MakerOrderExecutor.ActiveMakerOrders().NumOfOrders()
	if numOfMakerOrders > 0 {
		log.Warnf("maker orders are not all canceled")
		return
	}

	// if it's disconnected or context is canceled, then return
	select {
	case <-ctx.Done():
		return
	case <-s.makerConnectivity.DisconnectedC():
		return
	default:
	}

	bestBid, bestAsk, hasPrice := s.sourceBook.BestBidAndAsk()
	if !hasPrice {
		return
	}

	bestBidPrice := bestBid.Price
	bestAskPrice := bestAsk.Price
	log.Infof("%s book ticker: best ask / best bid = %v / %v", s.HedgeSymbol, bestAskPrice, bestBidPrice)

	s.lastSourcePrice.Set(bestBidPrice.Add(bestAskPrice).Div(Two))

	bookLastUpdateTime := s.sourceBook.LastUpdateTime()

	if _, err := s.bidPriceHeartBeat.Update(bestBid); err != nil {
		log.WithError(err).Warnf("quote update error, %s price not updating, order book last update: %s ago",
			s.Symbol,
			time.Since(bookLastUpdateTime))
	}

	if _, err := s.askPriceHeartBeat.Update(bestAsk); err != nil {
		log.WithError(err).Warnf("quote update error, %s price not updating, order book last update: %s ago",
			s.Symbol,
			time.Since(bookLastUpdateTime))
	}

	balances, err := s.MakerOrderExecutor.Session().Exchange.QueryAccountBalances(ctx)
	if err != nil {
		s.logger.WithError(err).Errorf("balance query error")
		return
	}

	log.Infof("balances: %+v", balances.NotZero())

	quoteBalance, ok := balances[s.makerMarket.QuoteCurrency]
	if !ok {
		return
	}

	baseBalance, ok := balances[s.makerMarket.BaseCurrency]
	if !ok {
		return
	}

	s.logger.Infof("quote balance: %s, base balance: %s", quoteBalance, baseBalance)

	submitOrders, err := s.generateMakerOrders(s.sourceBook, maxLayer, baseBalance.Available, quoteBalance.Available)
	if err != nil {
		s.logger.WithError(err).Errorf("generate order error")
		return
	}

	if len(submitOrders) == 0 {
		s.logger.Warnf("no orders are generated")
		return
	}

	_, err = s.MakerOrderExecutor.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		s.logger.WithError(err).Errorf("submit order error: %s", err.Error())
		return
	}
}

func (s *Strategy) cleanUpOpenOrders(ctx context.Context, session *bbgo.ExchangeSession) error {
	openOrders, err := retry.QueryOpenOrdersUntilSuccessful(ctx, session.Exchange, s.Symbol)
	if err != nil {
		return err
	}

	if len(openOrders) == 0 {
		return nil
	}

	return tradingutil.UniversalCancelAllOrders(ctx, session.Exchange, s.Symbol, openOrders)
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

func min(a, b int) int {
	if a < b {
		return a
	}

	return b
}

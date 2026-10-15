package xmaker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

const TradeTagMock = "mock"

// SyntheticHedge is a strategy that uses synthetic hedging to manage risk
// SourceSymbol could be something like binance.BTCUSDT
// FiatSymbol could be something like max.USDTTWD
//
// SplitSource can be used instead of Source to split the source hedge across multiple source markets,
// e.g., binance.BTCUSDT and okex.BTCUSDT, while the fiat position is still hedged on the single fiat market.
type SyntheticHedge struct {
	// SyntheticHedge is a strategy that uses synthetic hedging to manage risk
	Enabled bool `json:"enabled"`

	Source *HedgeMarketConfig `json:"source"`
	Fiat   *HedgeMarketConfig `json:"fiat"`

	// SplitSource splits the source hedge across multiple source markets.
	// It's mutually exclusive with Source.
	SplitSource *SplitHedge `json:"splitSource,omitempty"`

	// sourcePosition is the aggregated source position of all the split source markets.
	// It's only used when SplitSource is enabled.
	sourcePosition *types.Position

	sourceMarket, fiatMarket *HedgeMarket

	syntheticTradeId uint64

	logger logrus.FieldLogger

	mu sync.Mutex

	environment *bbgo.Environment

	strategy *Strategy

	forward bool
}

func (s *SyntheticHedge) isSplitSource() bool {
	return s.SplitSource != nil && s.SplitSource.Enabled
}

// sourceMarkets returns the source hedge markets sorted by the instance ID
func (s *SyntheticHedge) sourceMarkets() []*HedgeMarket {
	if !s.isSplitSource() {
		if s.sourceMarket == nil {
			return nil
		}

		return []*HedgeMarket{s.sourceMarket}
	}

	markets := make([]*HedgeMarket, 0, len(s.SplitSource.hedgeMarketInstances))
	for _, m := range s.SplitSource.hedgeMarketInstances {
		markets = append(markets, m)
	}

	sort.Slice(markets, func(i, j int) bool {
		return markets[i].InstanceID() < markets[j].InstanceID()
	})

	return markets
}

// getSourcePosition returns the position that is used for converting the fiat trades back to the source trades
func (s *SyntheticHedge) getSourcePosition() *types.Position {
	if s.isSplitSource() {
		return s.sourcePosition
	}

	return s.sourceMarket.Position
}

func (s *SyntheticHedge) sourcePositionStoreID() string {
	return "syntheticHedge-sourcePosition-" + s.sourcePosition.Symbol
}

// InitializeAndBind
// not a good way to initialize the synthetic hedge with the strategy instance
// but we need to build the trade collector to update the profit and position
func (s *SyntheticHedge) InitializeAndBind(sessions map[string]*bbgo.ExchangeSession, strategy *Strategy) error {
	if !s.Enabled {
		return nil
	}

	// Initialize the synthetic quote
	if s.Fiat == nil {
		return fmt.Errorf("fiat must be set")
	}

	// exclusive check for source and splitSource
	if s.isSplitSource() {
		if s.Source != nil {
			return fmt.Errorf("source and splitSource are mutually exclusive")
		}
	} else if s.Source == nil {
		return fmt.Errorf("source or splitSource must be set")
	}

	// initialize the logger
	if strategy != nil && strategy.logger != nil {
		s.logger = strategy.logger.WithFields(logrus.Fields{
			"feature": "synthetic_hedge",
		})
	} else {
		s.logger = log.WithFields(logrus.Fields{
			"feature": "synthetic_hedge",
		})
	}

	var err error

	if !s.isSplitSource() {
		// the ordinary source market initialization for synthetic hedge
		s.sourceMarket, err = InitializeHedgeMarketFromConfig(s.Source, sessions)
		if err != nil {
			return err
		}

		s.sourceMarket.SetLogger(s.logger)
		s.sourceMarket.dryRun = strategy.DryRun

		// update strategy instance ID for the source market
		s.sourceMarket.Position.StrategyInstanceID = strategy.InstanceID()
		s.sourceMarket.positionExposure.OnClose(strategy.positionExposure.Close)
	} else {
		// the split hedge binds the position exposure and the redispatch callbacks to the strategy,
		// but the source trades are handled by the synthetic hedge instead of the strategy trade collector.
		if err := s.SplitSource.initializeHedgeMarkets(sessions, strategy); err != nil {
			return err
		}
	}

	// ensure that at least one source market is initialized
	if len(s.sourceMarkets()) == 0 {
		return fmt.Errorf("syntheticHedge: no source market is initialized")
	}

	s.fiatMarket, err = InitializeHedgeMarketFromConfig(s.Fiat, sessions)
	if err != nil {
		return err
	}

	s.fiatMarket.SetLogger(s.logger)
	s.fiatMarket.dryRun = strategy.DryRun
	s.fiatMarket.Position.StrategyInstanceID = strategy.InstanceID()

	if err := s.initialize(strategy); err != nil {
		return err
	}

	// resolve the primary source market, validate the source markets against each other and the
	// fiat market, determine the conversion direction, and build the aggregated source position
	// for the split source if applicable.
	if err := s.initializeSourcePosition(); err != nil {
		return err
	}

	return nil
}

// initializeSourcePosition resolves the primary source market, validates the source markets
// against each other and the fiat market, determines the conversion direction and builds the
// aggregated source position for the split source.
func (s *SyntheticHedge) initializeSourcePosition() error {
	sourceMarkets := s.sourceMarkets()
	if len(sourceMarkets) == 0 {
		return fmt.Errorf("syntheticHedge: no source market is initialized")
	}

	// if the split source is enabled -> s.sourceMarket is not set yet
	if s.sourceMarket == nil {
		s.sourceMarket = sourceMarkets[0]
	}

	// ensure that all source markets have the same base and quote currencies
	for _, m := range sourceMarkets[1:] {
		if m.market.BaseCurrency != s.sourceMarket.market.BaseCurrency || m.market.QuoteCurrency != s.sourceMarket.market.QuoteCurrency {
			return fmt.Errorf("syntheticHedge: source market %s (%s/%s) does not match source market %s (%s/%s)",
				m.InstanceID(), m.market.BaseCurrency, m.market.QuoteCurrency,
				s.sourceMarket.InstanceID(), s.sourceMarket.market.BaseCurrency, s.sourceMarket.market.QuoteCurrency)
		}
	}

	sourceMarket := s.sourceMarket.market
	fiatMarket := s.fiatMarket.market

	// BTC/USDT -> USDT/TWD = forward
	// BTC/USDT -> USDC/USDT = reverse (!forward)
	s.forward = sourceMarket.QuoteCurrency == fiatMarket.BaseCurrency

	if s.isSplitSource() {
		if sourceMarket.QuoteCurrency != fiatMarket.BaseCurrency && sourceMarket.QuoteCurrency != fiatMarket.QuoteCurrency {
			return fmt.Errorf("syntheticHedge: source quote currency %s does not match fiat market %s",
				sourceMarket.QuoteCurrency, fiatMarket.Symbol)
		}
	}

	// always initialize sourcePosition so it's never nil, even though it's only
	// read/written when split source is enabled (see getSourcePosition, isSplitSource).
	s.sourcePosition = types.NewPositionFromMarket(sourceMarket)
	s.sourcePosition.Strategy = ID
	if s.strategy != nil {
		s.sourcePosition.StrategyInstanceID = s.strategy.InstanceID()
	}

	return nil
}

func (s *SyntheticHedge) initialize(strategy *Strategy) error {
	s.strategy = strategy
	s.environment = strategy.Environment

	for _, sourceMarket := range s.sourceMarkets() {
		sourceMarket.tradeCollector.OnTrade(func(trade types.Trade, _, _ fixedpoint.Value) {
			s.handleSourceTrade(trade)
		})
	}

	s.fiatMarket.tradeCollector.OnTrade(func(trade types.Trade, _, _ fixedpoint.Value) {
		s.handleFiatTrade(strategy, trade)
	})

	return nil
}

// Hedge is the main function to perform the synthetic hedging:
// 1) use the snapshot price as the source average cost
// 2) submit the hedge order to the source makerExchange
// 3) query trades from of the hedge order.
// 4) build up the source hedge position for the average cost.
// 5) submit fiat hedge order to the fiat market to convert the quote.
// 6) merge the positions.
func (s *SyntheticHedge) Hedge(
	ctx context.Context,
	uncoveredPosition, hedgeDelta fixedpoint.Value,
) error {
	s.logger.Infof("[syntheticHedge] synthetic hedging with delta: %f", uncoveredPosition.Float64())

	if s.isSplitSource() {
		// the split hedge covers the strategy position exposure for each split
		return s.SplitSource.Hedge(ctx, uncoveredPosition, hedgeDelta)
	}

	s.strategy.positionExposure.Cover(uncoveredPosition)

	s.sourceMarket.positionDeltaC <- uncoveredPosition
	return nil
}

func (s *SyntheticHedge) GetQuotePrices() (fixedpoint.Value, fixedpoint.Value, bool) {
	if s.fiatMarket == nil {
		return fixedpoint.Zero, fixedpoint.Zero, false
	}

	var bid, ask fixedpoint.Value
	if s.isSplitSource() {
		var ok bool
		bid, ask, ok = s.SplitSource.GetBalanceWeightedQuotePrice()
		if !ok {
			return fixedpoint.Zero, fixedpoint.Zero, false
		}
	} else {
		if s.sourceMarket == nil {
			return fixedpoint.Zero, fixedpoint.Zero, false
		}

		bid, ask = s.sourceMarket.GetQuotePrice()
	}

	bid2, ask2 := s.fiatMarket.GetQuotePrice()

	if s.forward {
		sBid := bid.Mul(bid2)
		sAsk := ask.Mul(ask2)

		log.Infof("synthetic quote:[%f/%f] source:[%f/%f] fiat:[%f/%f]",
			sAsk.Float64(), sBid.Float64(),
			ask.Float64(), bid.Float64(),
			ask2.Float64(), bid2.Float64())
		return sBid, sAsk, sBid.Sign() > 0 && sAsk.Sign() > 0
	} else {
		sBid := bid.Div(bid2)
		sAsk := ask.Div(ask2)

		log.Infof("synthetic quote:[%f/%f] source:[%f/%f] fiat:[%f/%f]",
			sAsk.Float64(), sBid.Float64(),
			ask.Float64(), bid.Float64(),
			ask2.Float64(), bid2.Float64())
		return sBid, sAsk, sBid.Sign() > 0 && sAsk.Sign() > 0
	}
}

func (s *SyntheticHedge) Stop(shutdownCtx context.Context) error {
	s.logger.Infof("[syntheticHedge] stopping synthetic hedge workers")
	if s.isSplitSource() {
		if err := s.SplitSource.Stop(shutdownCtx); err != nil {
			s.logger.WithError(err).Errorf("[syntheticHedge] failed to stop split source markets")
		}
	} else {
		s.sourceMarket.Stop(shutdownCtx)
	}

	s.fiatMarket.Stop(shutdownCtx)

	instanceID := ID + "-synthetichedge"
	if s.strategy != nil {
		instanceID = s.strategy.InstanceID()
	}

	for _, sourceMarket := range s.sourceMarkets() {
		sourceMarket.Sync(sourceMarket.tradingCtx, instanceID)
	}

	s.fiatMarket.Sync(s.fiatMarket.tradingCtx, instanceID)

	if s.isSplitSource() {
		s.syncSourcePosition(s.fiatMarket.tradingCtx, instanceID)
	}

	s.logger.Infof("[syntheticHedge] synthetic hedge workers stopped")
	return nil
}

func (s *SyntheticHedge) Start(ctx context.Context) error {
	if !s.Enabled {
		return nil
	}

	sourceMarkets := s.sourceMarkets()
	if len(sourceMarkets) == 0 || s.fiatMarket == nil {
		return fmt.Errorf("sourceMarket and fiatMarket must be initialized")
	}

	instanceID := ID
	if s.strategy != nil {
		instanceID = s.strategy.InstanceID()
	}

	for _, sourceMarket := range sourceMarkets {
		if err := sourceMarket.Restore(ctx, instanceID); err != nil {
			s.logger.WithError(err).Errorf("[syntheticHedge] failed to restore source market %s persistence", sourceMarket.InstanceID())
			return err
		}
	}

	if err := s.restoreSourcePosition(ctx, instanceID); err != nil {
		s.logger.WithError(err).Errorf("[syntheticHedge] failed to restore source position persistence")
		return err
	}

	if err := s.fiatMarket.Restore(ctx, instanceID); err != nil {
		s.logger.WithError(err).Errorf("[syntheticHedge] failed to restore fiat market persistence")
		return err
	}

	if s.isSplitSource() {
		// SplitHedge.Start also waits for the source markets to be ready
		if err := s.SplitSource.Start(ctx); err != nil {
			s.logger.WithError(err).Errorf("[syntheticHedge] failed to start split source markets")
			return err
		}
	} else {
		if err := s.sourceMarket.Start(ctx); err != nil {
			s.logger.WithError(err).Errorf("[syntheticHedge] failed to start")
			return err
		}
	}

	if err := s.fiatMarket.Start(ctx); err != nil {
		s.logger.WithError(err).Errorf("[syntheticHedge] failed to start")
		return err
	}

	if !s.isSplitSource() {
		s.sourceMarket.WaitForReady(ctx)
	}

	s.fiatMarket.WaitForReady(ctx)

	s.logger.Infof("[syntheticHedge] source market and fiat market are ready")
	return nil
}

// restoreSourcePosition loads the aggregated source position of the split source from persistence
func (s *SyntheticHedge) restoreSourcePosition(ctx context.Context, namespace string) error {
	isolation := bbgo.GetIsolationFromContext(ctx)
	ps := isolation.GetPersistenceService()
	store := ps.NewStore(namespace, s.sourcePositionStoreID())

	if err := store.Load(&s.sourcePosition); err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, service.ErrPersistenceNotExists) {
			return nil
		}

		return fmt.Errorf("failed to load synthetic hedge source position: %w", err)
	}

	s.logger.Infof("[syntheticHedge] restored source position: %+v", s.sourcePosition)
	return nil
}

// syncSourcePosition saves the aggregated source position of the split source to persistence
func (s *SyntheticHedge) syncSourcePosition(ctx context.Context, namespace string) {
	isolation := bbgo.GetIsolationFromContext(ctx)
	ps := isolation.GetPersistenceService()
	store := ps.NewStore(namespace, s.sourcePositionStoreID())

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := store.Save(s.sourcePosition); err != nil {
		s.logger.WithError(err).Errorf("[syntheticHedge] failed to save source position")
	}
}

func newMockOrderFromTrade(trade types.Trade, orderType types.OrderType) types.Order {
	return types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:       trade.Symbol,
			Side:         trade.Side,
			Type:         orderType,
			Quantity:     trade.Quantity,
			Price:        trade.Price,
			AveragePrice: fixedpoint.Zero,
			StopPrice:    fixedpoint.Zero,
		},
		Exchange:         trade.Exchange,
		OrderID:          trade.OrderID,
		Status:           types.OrderStatusFilled,
		ExecutedQuantity: trade.Quantity,
		IsWorking:        false,
		CreationTime:     trade.Time,
		UpdateTime:       trade.Time,
	}
}

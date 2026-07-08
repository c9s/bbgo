package xfundingv2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/datasource/coinmarketcap"
	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/risk/circuitbreaker"
	"github.com/c9s/bbgo/pkg/slack/slackalert"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

const ID = "xfundingv2"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type CriticalErrorConfig struct {
	MaxRemainingNotional fixedpoint.Value `json:"maxRemainingNotional"`
	MaxFundingRateFlip   fixedpoint.Value `json:"maxFundingRateFlip"`
	MaxMoqDeviation      fixedpoint.Value `json:"maxMoqDeviation"`
}

func (c *CriticalErrorConfig) Defaults() {
	if c.MaxRemainingNotional.IsZero() {
		c.MaxRemainingNotional = fixedpoint.NewFromInt(500)
	}
	if c.MaxFundingRateFlip.IsZero() {
		c.MaxFundingRateFlip = fixedpoint.NewFromFloat(0.0003) // 0.03%
	}
	if c.MaxMoqDeviation.IsZero() {
		// max tolerance of 5x of the minium order quantity of the market.
		c.MaxMoqDeviation = fixedpoint.NewFromInt(5)
	}
}

type Strategy struct {
	Environment *bbgo.Environment

	// strategy mutex
	// lock before update the strategy state, such as current round, selected market, etc
	mu sync.Mutex

	DryRun              bool `json:"dryRun"`
	ClosingAllOnStartup bool `json:"closingAllOnStartup"`

	// Session configuration
	SpotSession    string `json:"spotSession"`
	FuturesSession string `json:"futuresSession"`

	// CandidateSymbols is the list of symbols to consider for selection
	// IMPORTANT: xfundingv2 is now assuming trading on U-major pairs
	CandidateSymbols     []string       `json:"candidateSymbols"`
	OpenPositionInterval types.Duration `json:"openPositionInterval"`
	TransitRoundInterval types.Duration `json:"transitRoundInterval"`

	// TickSymbol is the symbol used for ticking the strategy, default to the first candidate symbol
	TickSymbol   string         `json:"tickSymbol"`
	TickInterval types.Interval `json:"tickInterval"`

	FeeSymbol       string                      `json:"feeSymbol"`
	FeeDiscountRate map[string]fixedpoint.Value `json:"feeDiscountRate"`
	QuoteCurrency   string                      `json:"quoteCurrency"`

	PendingRoundGracePeriod types.Duration   `json:"pendingRoundGracePeriod"`
	MaxPendingRoundRetry    int              `json:"maxPendingRoundRetry"`
	TWAPWorkerConfig        TWAPWorkerConfig `json:"twap"`

	// Market selection criteria
	MarketSelectionConfig *MarketSelectionConfig      `json:"marketSelection,omitempty"`
	MaxPositionExposure   map[string]fixedpoint.Value `json:"maxPositionExposure"`

	HaltRoundNotificationInterval types.Duration `json:"haltRoundNotificationInterval"`
	haltNotificationLimiters      map[string]*rate.Limiter

	CriticalErrorConfig CriticalErrorConfig                            `json:"criticalErrorConfig"`
	CircuitBreakers     map[string]*circuitbreaker.BasicCircuitBreaker `json:"circuitBreakers"`

	SlackAlert slackalert.SlackAlert `json:"slackAlert"`

	Leverage int `json:"leverage"`

	spotSession, futuresSession *bbgo.ExchangeSession
	futuresService              FuturesService

	// stream order books to keep track of the latest order book for the candidate symbols
	futuresOrderBooks, spotOrderBooks map[string]*types.StreamOrderBook

	candidateSymbols          []string
	costEstimator             *CostEstimator
	preliminaryMarketSelector *MarketSelector

	coinmarketcapClient *coinmarketcap.DataSource

	// persistence states
	// PendingRounds are the rounds that are waiting for the preparation work to be done
	// before entering the active round list, such as fee asset preparation, transfer, etc.
	PendingRounds map[string]*PendingRound `persistence:"pendingRounds"`

	// ActiveRounds are the rounds that are actively trading or holding positions.
	// rounds in the active list should be at the state of either RoundOpening, RoundReady or RoundClosing.
	ActiveRounds map[string]*ArbitrageRound `persistence:"activeRounds"`

	// closed rounds that are waiting for cleanup
	// the cleanup process is to
	// 1. ensure that the positions will restore to zero
	// 2. all open orders are canceled.
	// 3. collaterals are transferred back to the spot account.
	MaxClosedRetryCnt int                        `json:"maxClosedRetryCnt"`
	ClosedRoundTasks  map[string]*CloseRoundTask `persistence:"closedRounds"`

	// the positions are shared across rounds and the executors of the same symbol.
	SpotPositions    map[string]*types.Position `persistence:"spotPositions"`
	FuturesPositions map[string]*types.Position `persistence:"futuresPositions"`

	// StatsPeriod controls the duration of logging the stats of the strategy, such as the number of active rounds, pending rounds, closed rounds, etc.
	StatsPeriod   types.Duration `json:"statsPeriod"`
	lastStatsTime time.Time

	// order executors for each symbol
	// we need to cache the executors as map at startup since the executors are bound to the user data stream (via `.Bind()`).
	// if we do no reuse them and create new executor at each round, the callbacks of the user data stream will be full of stale executors.
	spotGeneralOrderExecutors    map[string]*bbgo.GeneralOrderExecutor
	futuresGeneralOrderExecutors map[string]*bbgo.GeneralOrderExecutor

	logger     logrus.FieldLogger
	logLimiter *rate.Limiter

	lastTickTime time.Time
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	symbols := strings.Join(s.CandidateSymbols, "_")
	return fmt.Sprintf("%s-%s-%s-futures", ID, symbols, s.MarketSelectionConfig.FuturesDirection)
}

func (s *Strategy) Defaults() error {
	if len(s.CandidateSymbols) == 0 {
		return errors.New("empty candidateSymbols")
	}

	s.TWAPWorkerConfig.Defaults()

	if s.TickSymbol == "" {
		s.TickSymbol = s.CandidateSymbols[0]
	}
	if s.TickInterval.Duration() == 0 {
		s.TickInterval = types.Interval1m
	}
	if s.OpenPositionInterval.Duration() == 0 {
		s.OpenPositionInterval = types.Duration(time.Minute * 30)
	}
	if s.TransitRoundInterval.Duration() == 0 {
		s.TransitRoundInterval = types.Duration(time.Minute * 10)
	}

	if s.MarketSelectionConfig == nil {
		s.MarketSelectionConfig = &MarketSelectionConfig{}
	}
	s.MarketSelectionConfig.Defaults()
	if s.QuoteCurrency == "" {
		s.QuoteCurrency = "USDT"
	}

	if s.PendingRoundGracePeriod.Duration() == 0 {
		s.PendingRoundGracePeriod = types.Duration(time.Minute * 10)
	}
	if s.MaxPendingRoundRetry == 0 {
		s.MaxPendingRoundRetry = 3
	}

	if s.MaxClosedRetryCnt == 0 {
		s.MaxClosedRetryCnt = 3
	}

	if s.HaltRoundNotificationInterval.Duration() == 0 {
		s.HaltRoundNotificationInterval = types.Duration(time.Minute * 30)
	}
	s.CriticalErrorConfig.Defaults()

	if s.StatsPeriod.Duration() == 0 {
		s.StatsPeriod = types.Duration(time.Hour * 2)
	}

	if s.FeeDiscountRate == nil {
		s.FeeDiscountRate = make(map[string]fixedpoint.Value)
	}
	if _, ok := s.FeeDiscountRate["spot"]; !ok {
		s.FeeDiscountRate["spot"] = fixedpoint.NewFromFloat(0.25)
	}
	if _, ok := s.FeeDiscountRate["futures"]; !ok {
		s.FeeDiscountRate["futures"] = fixedpoint.NewFromFloat(0.10)
	}

	if s.Leverage == 0 {
		s.Leverage = 2
	}

	return nil
}

func (s *Strategy) Initialize() error {
	if os.Getenv("DEBUG_XFUNDINGV2") != "" {
		s.logger = s.newDebugLogger()
	} else {
		s.logger = logrus.WithFields(logrus.Fields{
			"strategy":    ID,
			"strategy_id": s.InstanceID(),
		})
	}

	if apiKey := os.Getenv("COINMARKETCAP_API_KEY"); apiKey == "" {
		s.logger.Warn("CoinMarketCap API key not set, top cap market filtering will be disabled")
	} else {
		s.coinmarketcapClient = coinmarketcap.New(apiKey)
	}
	s.futuresOrderBooks = make(map[string]*types.StreamOrderBook)
	s.spotOrderBooks = make(map[string]*types.StreamOrderBook)

	// Initialize executor maps
	s.spotGeneralOrderExecutors = make(map[string]*bbgo.GeneralOrderExecutor)
	s.futuresGeneralOrderExecutors = make(map[string]*bbgo.GeneralOrderExecutor)
	if !bbgo.IsBackTesting {
		s.logLimiter = rate.NewLimiter(rate.Every(time.Minute*10), 1)
	}
	if s.MaxPositionExposure == nil {
		s.MaxPositionExposure = make(map[string]fixedpoint.Value)
	}
	if s.CircuitBreakers == nil {
		s.CircuitBreakers = make(map[string]*circuitbreaker.BasicCircuitBreaker)
	}
	s.haltNotificationLimiters = make(map[string]*rate.Limiter)
	return nil
}

func (s *Strategy) Validate() error {
	if len(s.CandidateSymbols) == 0 {
		return errors.New("candidateSymbols is required")
	}
	for symbol, maxExposure := range s.MaxPositionExposure {
		if maxExposure.Sign() < 0 {
			return fmt.Errorf("maxPositionExposure should be positive: %s", symbol)
		}
	}
	for feeType, rate := range s.FeeDiscountRate {
		if rate.Sign() < 0 || rate.Compare(fixedpoint.One) > 0 {
			return fmt.Errorf("feeDiscountRate[%s] should be between 0 and 1: %s", feeType, rate)
		}
	}
	return nil
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	spotSession, ok := sessions[s.SpotSession]
	if !ok {
		s.logger.Warnf("spot session %s not found, skip subscription", s.SpotSession)
		return
	}
	futuresSession, ok := sessions[s.FuturesSession]
	if !ok {
		s.logger.Warnf("futures session %s not found, skip subscription", s.SpotSession)
		return
	}
	// subscribe kline events for ticking the strategy
	spotSession.Subscribe(types.KLineChannel, s.TickSymbol, types.SubscribeOptions{Interval: s.TickInterval})
	// subscribe kline events for updating the tick price of the active rounds
	for _, symbol := range s.CandidateSymbols {
		futuresSession.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{Interval: s.TickInterval})
	}
}

func (s *Strategy) CrossRun(
	ctx context.Context, _ bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession,
) error {
	// Initialize position maps (may be populated by LoadState if persisted state exists)
	if s.SpotPositions == nil {
		s.SpotPositions = make(map[string]*types.Position)
	}
	if s.FuturesPositions == nil {
		s.FuturesPositions = make(map[string]*types.Position)
	}

	// Initialize round maps (may be populated by LoadState if persisted state exists)
	if s.ActiveRounds == nil {
		s.ActiveRounds = make(map[string]*ArbitrageRound)
	}
	if s.PendingRounds == nil {
		s.PendingRounds = make(map[string]*PendingRound)
	}
	if s.ClosedRoundTasks == nil {
		s.ClosedRoundTasks = make(map[string]*CloseRoundTask)
	}
	s.logger.Debugf("active rounds: %d, pending rounds: %d, closed rounds: %d", len(s.ActiveRounds), len(s.PendingRounds), len(s.ClosedRoundTasks))

	s.spotSession = sessions[s.SpotSession]
	s.futuresSession = sessions[s.FuturesSession]

	if s.futuresSession == nil {
		return fmt.Errorf("futures session %s not found", s.FuturesSession)
	}
	if s.spotSession == nil {
		return fmt.Errorf("spot session %s not found", s.SpotSession)
	}
	if futuresEx, ok := s.futuresSession.Exchange.(types.FuturesExchange); !ok {
		return fmt.Errorf("session %s does not support futures", s.futuresSession.Name)
	} else if !futuresEx.GetFuturesSettings().IsFutures {
		return fmt.Errorf("session %s is not configured for futures trading", s.futuresSession.Name)
	}

	if err := s.checkAndFixMarginMode(ctx); err != nil {
		return fmt.Errorf("failed to check and fix margin mode: %w", err)
	}

	// extra setup for Binance exchange
	if err := s.setupBinance(ctx, s.futuresSession); err != nil {
		return err
	}
	if err := s.setupBinance(ctx, s.spotSession); err != nil {
		return err
	}
	s.logger.Infof("fee symbol: %s, fee discount rate: spot=%s futures=%s",
		s.FeeSymbol,
		s.FeeDiscountRate["spot"].Percentage(),
		s.FeeDiscountRate["futures"].Percentage())

	if futuresService, ok := s.futuresSession.Exchange.(FuturesService); !ok {
		return fmt.Errorf("futures session exchange does not support futures service: %s", s.futuresSession.ExchangeName)
	} else {
		s.futuresService = futuresService
	}

	if _, ok := s.spotSession.Exchange.(types.ExchangeOrderQueryService); !ok {
		return fmt.Errorf("spot session exchange does not support order query service: %s", s.spotSession.ExchangeName)
	}
	if _, ok := s.futuresSession.Exchange.(types.ExchangeOrderQueryService); !ok {
		return fmt.Errorf("futures session exchange does not support order query service: %s", s.futuresSession.ExchangeName)
	}

	// remaining open position check
	risks, err := s.futuresService.QueryPositionRisk(ctx)
	if err != nil {
		return fmt.Errorf("failed to query position risk from futures session exchange: %w", err)
	}
	risksMap := make(map[string]types.PositionRisk)
	for _, risk := range risks {
		risksMap[risk.Symbol] = risk
	}
	// if there is any open position, it should have a corresponding active round which is loaded via LoadState.
	// Otherwise, it is a mismatch and should raise an error to stop the strategy from running.
	var mismatchSymbols []string
	for _, risk := range risks {
		_, found := s.ActiveRounds[risk.Symbol]
		if !risk.PositionAmount.IsZero() && !found {
			mismatchSymbols = append(mismatchSymbols, risk.Symbol)
		}
	}
	if len(mismatchSymbols) > 0 {
		return fmt.Errorf("found open positions without active rounds: %v on %s", mismatchSymbols, s.futuresSession.Exchange.Name())
	}

	// initialize cost estimator
	futuresFeeRate := types.ExchangeFee{
		MakerFeeRate: s.futuresSession.MakerFeeRate,
		TakerFeeRate: s.futuresSession.TakerFeeRate,
	}
	spotFeeRate := types.ExchangeFee{
		MakerFeeRate: s.spotSession.MakerFeeRate,
		TakerFeeRate: s.spotSession.TakerFeeRate,
	}
	if r, ok := s.FeeDiscountRate["futures"]; ok && !r.IsZero() {
		discountFactor := fixedpoint.One.Sub(r)
		futuresFeeRate.MakerFeeRate = futuresFeeRate.MakerFeeRate.Mul(discountFactor)
		futuresFeeRate.TakerFeeRate = futuresFeeRate.TakerFeeRate.Mul(discountFactor)
	}
	if r, ok := s.FeeDiscountRate["spot"]; ok && !r.IsZero() {
		discountFactor := fixedpoint.One.Sub(r)
		spotFeeRate.MakerFeeRate = spotFeeRate.MakerFeeRate.Mul(discountFactor)
		spotFeeRate.TakerFeeRate = spotFeeRate.TakerFeeRate.Mul(discountFactor)
	}
	s.costEstimator = NewCostEstimator()
	s.costEstimator.
		SetFuturesFeeRate(futuresFeeRate).
		SetSpotFeeRate(spotFeeRate)

	// static filters
	var candidateSymbols []string
	// 1. should be listed on both spot and futures
	candidateSymbols = s.filterMarketBothListed(s.CandidateSymbols)
	// 2. filter by collateral rate
	candidateSymbols = s.filterMarketCollateralRate(ctx, candidateSymbols)
	// 3. filter by top N market cap
	candidateSymbols = s.filterMarketByCapSize(ctx, candidateSymbols)

	if len(candidateSymbols) == 0 {
		return errors.New("no candidate symbols after filtering")
	}
	// market checking and setup the general order executors
	// NOTE: the executors should be created first before anything else to ensure the executors get updated first.
	// The twap executors rely on the state of the general order executors to determine the filled quantity.
	s.candidateSymbols = candidateSymbols
	// set leverage for all candidate symbols
	if err := s.setLeverage(ctx); err != nil {
		return err
	}
	setupGeneralExecutorsForSymbol := func(symbol string) error {
		spotMarket, found := s.spotSession.Market(symbol)
		if !found {
			return fmt.Errorf("market %s not found in spot session", symbol)
		}
		futuresMarket, found := s.futuresSession.Market(symbol)
		if !found {
			return fmt.Errorf("market %s not found in futures session", symbol)
		}
		// check all symbols have the same quote currency
		if spotMarket.QuoteCurrency != s.QuoteCurrency {
			return fmt.Errorf("spot market %s quote currency %s does not match strategy quote currency %s",
				symbol, spotMarket.QuoteCurrency, s.QuoteCurrency)
		}
		if futuresMarket.QuoteCurrency != s.QuoteCurrency {
			return fmt.Errorf("futures market %s quote currency %s does not match strategy quote currency %s",
				symbol, futuresMarket.QuoteCurrency, s.QuoteCurrency)
		}

		var spotPosition, futuresPosition *types.Position
		if p, found := s.SpotPositions[symbol]; found {
			spotPosition = p
		} else {
			spotPosition = types.NewPositionFromMarket(spotMarket)
			s.SpotPositions[symbol] = spotPosition
		}
		if p, found := s.FuturesPositions[symbol]; found {
			futuresPosition = p
		} else {
			futuresPosition = types.NewPositionFromMarket(futuresMarket)
			s.FuturesPositions[symbol] = futuresPosition
		}
		spotExecutor := bbgo.NewGeneralOrderExecutor(
			s.spotSession,
			symbol,
			s.ID(),
			s.InstanceID(),
			spotPosition,
		)
		spotExecutor.OrderStore().AddOrderUpdate = true
		spotExecutor.DisableNotify()
		spotExecutor.Bind()
		if openOrders, err := s.spotSession.Exchange.QueryOpenOrders(ctx, symbol); err != nil {
			return fmt.Errorf("failed to query open orders for spot symbol %s: %w", symbol, err)
		} else if len(openOrders) > 0 {
			spotExecutor.ActiveMakerOrders().Add(openOrders...)
		}
		s.spotGeneralOrderExecutors[symbol] = spotExecutor
		futuresExecutor := bbgo.NewGeneralOrderExecutor(
			s.futuresSession,
			symbol,
			s.ID(),
			s.InstanceID(),
			futuresPosition,
		)
		if openOrders, err := s.futuresSession.Exchange.QueryOpenOrders(ctx, symbol); err != nil {
			return fmt.Errorf("failed to query open orders for futures symbol %s: %w", symbol, err)
		} else if len(openOrders) > 0 {
			futuresExecutor.ActiveMakerOrders().Add(openOrders...)
		}
		futuresExecutor.OrderStore().AddOrderUpdate = true
		futuresExecutor.DisableNotify()
		futuresExecutor.Bind()
		s.futuresGeneralOrderExecutors[symbol] = futuresExecutor
		return nil
	}
	for _, symbol := range s.candidateSymbols {
		if err := setupGeneralExecutorsForSymbol(symbol); err != nil {
			return err
		}
	}

	if s.FeeSymbol != "" {
		feeMarket, found := s.spotSession.Market(s.FeeSymbol)
		if !found {
			return fmt.Errorf("fee market %s not found in spot session", s.FeeSymbol)
		}
		if feeMarket.QuoteCurrency != s.QuoteCurrency {
			return fmt.Errorf("fee market %s quote currency %s does not match strategy quote currency %s",
				s.FeeSymbol, feeMarket.QuoteCurrency, s.QuoteCurrency)
		}
		var spotPosition *types.Position
		if p, found := s.SpotPositions[s.FeeSymbol]; found {
			spotPosition = p
		} else {
			spotPosition = types.NewPositionFromMarket(feeMarket)
			s.SpotPositions[s.FeeSymbol] = spotPosition
		}
		spotExecutor := bbgo.NewGeneralOrderExecutor(
			s.spotSession,
			s.FeeSymbol,
			s.ID(),
			s.InstanceID(),
			spotPosition,
		)
		spotExecutor.DisableNotify()
		spotExecutor.Bind()
		s.spotGeneralOrderExecutors[s.FeeSymbol] = spotExecutor
	}

	if futuresInfoService, ok := s.futuresSession.Exchange.(FuturesInfoService); !ok {
		return fmt.Errorf("futures session exchange does not support futures info service: %s", s.futuresSession.ExchangeName)
	} else {
		s.preliminaryMarketSelector = NewMarketSelector(*s.MarketSelectionConfig, futuresInfoService, s.logger)
	}

	// initialize depth books for model selection
	// we create new stream here to save the bandwidth of the market data stream of the sessions
	futureStream := s.futuresSession.Exchange.NewStream()
	futureStream.SetPublicOnly()
	spotStream := s.spotSession.Exchange.NewStream()
	spotStream.SetPublicOnly()
	setupStreamBooksForSymbol := func(symbol string) {
		futuresBook := types.NewStreamBook(symbol, s.futuresSession.ExchangeName)
		futuresBook.BindStream(futureStream)
		futureStream.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{})
		s.futuresOrderBooks[symbol] = futuresBook

		spotBook := types.NewStreamBook(symbol, s.spotSession.ExchangeName)
		spotBook.BindStream(spotStream)
		spotStream.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{})
		s.spotOrderBooks[symbol] = spotBook
	}
	for _, symbol := range candidateSymbols {
		setupStreamBooksForSymbol(symbol)
	}
	// subscribe fee symbol order book for trading fee estimation
	if s.FeeSymbol != "" {
		spotBook := types.NewStreamBook(s.FeeSymbol, s.spotSession.ExchangeName)
		spotBook.BindStream(spotStream)
		spotStream.Subscribe(types.BookChannel, s.FeeSymbol, types.SubscribeOptions{})
		s.spotOrderBooks[s.FeeSymbol] = spotBook
	}
	// runtime init done, setup all rounds
	for _, round := range s.allRounds() {
		symbol := round.SpotSymbol()
		// setup order executors
		if _, found := s.spotGeneralOrderExecutors[symbol]; !found {
			if err := setupGeneralExecutorsForSymbol(symbol); err != nil {
				return fmt.Errorf("failed to setup order executors for pending round symbol %s: %w", symbol, err)
			}
		}
		// setup stream books for the symbols
		if _, found := s.spotOrderBooks[symbol]; !found {
			setupStreamBooksForSymbol(symbol)
		}
		if err := round.Initialize(ctx, s); err != nil {
			return fmt.Errorf("failed to restore round (%s): %w", round, err)
		}
		// circuit breaker setup
		if breaker, found := s.CircuitBreakers[symbol]; !found {
			s.CircuitBreakers[symbol] = s.defaultBreaker(symbol)
		} else {
			breaker.SetMetricsInfo(
				s.ID(),
				s.InstanceID(),
				symbol,
			)
		}
		round.SetSlackAlert(s.SlackAlert)
	}

	for _, symbol := range s.candidateSymbols {
		if breaker, found := s.CircuitBreakers[symbol]; !found {
			s.CircuitBreakers[symbol] = s.defaultBreaker(symbol)
		} else {
			breaker.SetMetricsInfo(
				s.ID(),
				s.InstanceID(),
				symbol,
			)
		}
	}

	for _, closedRound := range s.ClosedRoundTasks {
		// give the restored closed round one more chance to be processed.
		if closedRound.RetryCnt >= s.MaxClosedRetryCnt {
			closedRound.RetryCnt--
			closedRound.Notified = false
		}
	}

	// setup halt notification limiters
	for _, symbol := range s.candidateSymbols {
		s.haltNotificationLimiters[symbol] = rate.NewLimiter(
			rate.Every(s.HaltRoundNotificationInterval.Duration()),
			1,
		)
	}
	for _, round := range s.allRounds() {
		if _, found := s.haltNotificationLimiters[round.SpotSymbol()]; !found {
			s.haltNotificationLimiters[round.SpotSymbol()] = rate.NewLimiter(
				rate.Every(s.HaltRoundNotificationInterval.Duration()),
				1,
			)
		}
	}

	if err := futureStream.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect future stream books: %w", err)
	}
	if err := spotStream.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect spot stream books: %w", err)
	}

	if !bbgo.IsBackTesting {
		// simple hack to ensure the stream books are ready
		time.Sleep(10 * time.Second)
		// NOTE: must prepare the rounds after the stream books are connected
		if err := s.prepareRounds(ctx); err != nil {
			return err
		}
	}

	// setup metrics for positions
	for _, position := range s.SpotPositions {
		position.Strategy = s.ID()
		position.StrategyInstanceID = s.InstanceID()
	}
	for _, position := range s.FuturesPositions {
		position.Strategy = s.ID()
		position.StrategyInstanceID = s.InstanceID()
	}

	// setup callbacks
	s.spotSession.MarketDataStream.OnKLineClosed(types.KLineWith(s.TickSymbol, s.TickInterval, func(kline types.KLine) {
		s.tick(ctx, kline.EndTime.Time())
	}))

	// trade update callbacks
	s.spotSession.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
		// lock the strategy to ensure all the updates to the active rounds are seen
		s.mu.Lock()
		defer s.mu.Unlock()

		balance := s.spotSession.Account.Balances()
		for _, round := range s.allRounds() {
			if round.HasOrder(trade.OrderID) {
				round.HandleSpotTrade(trade, balance, trade.Time.Time())

				spotFilledPosition := round.SpotWorker().FilledPosition()
				filledRatio := spotFilledPosition.Div(round.TriggeredTargetPosition()).Abs()
				if round.State() == RoundClosing {
					filledRatio = fixedpoint.One.Sub(filledRatio)
				}
				roundPositionFilledRatioMetrics.With(
					prometheus.Labels{
						"strategy_id": s.InstanceID(),
						"symbol":      round.SpotSymbol(),
						"accountType": "spot",
					},
				).Set(filledRatio.Float64())
			}
		}
	})
	s.futuresSession.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
		s.mu.Lock()
		defer s.mu.Unlock()

		balance := s.futuresSession.Account.Balances()
		for _, round := range s.allRounds() {
			if round.HasOrder(trade.OrderID) {
				round.HandleFuturesTrade(trade, balance, trade.Time.Time())

				futuresFilledPosition := round.FuturesWorker().FilledPosition()
				filledRatio := futuresFilledPosition.Div(round.TriggeredTargetPosition()).Abs()
				if round.State() == RoundClosing {
					filledRatio = fixedpoint.One.Sub(filledRatio)
				}
				roundPositionFilledRatioMetrics.With(
					prometheus.Labels{
						"strategy_id": s.InstanceID(),
						"symbol":      round.SpotSymbol(),
						"accountType": "futures",
					},
				).Set(filledRatio.Float64())
			}
		}
	})

	// order update callbacks
	s.spotSession.UserDataStream.OnOrderUpdate(func(update types.Order) {
		round, found := s.ActiveRounds[update.Symbol]
		if !found {
			return
		}
		s.logger.Debugf("spot order update: %s", update)
		round.HandleSpotOrderUpdate(update)
		bbgo.Notify("📝 Round spot order update: %s", round.String(), update)
	})
	s.futuresSession.UserDataStream.OnOrderUpdate(func(update types.Order) {
		round, found := s.ActiveRounds[update.Symbol]
		if !found {
			return
		}
		s.logger.Debugf("futures order update: %s", update)
		round.HandleFuturesOrderUpdate(update)
		bbgo.Notify("📝 Round futures order update: %s", round.String(), update)
	})
	// market stream callbacks
	s.futuresSession.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if round, found := s.ActiveRounds[kline.Symbol]; found {
			round.SetTickPrice(kline.Close)
		}
	})

	// strategy is ready for running
	if !bbgo.IsBackTesting && s.ClosingAllOnStartup {
		s.mu.Lock()
		s.logger.Info("closing all active rounds and clearing pending rounds on startup")
		// set all active rounds to closing state and clear the pending rounds
		for _, round := range s.ActiveRounds {
			if round.State() == RoundClosing {
				continue
			}
			round.SetClosing(time.Now(), s.TWAPWorkerConfig.ClosingDuration)
			bbgo.Notify("⚠️ Round is set to closing state on startup", round.NewNotification())
		}
		s.PendingRounds = make(map[string]*PendingRound)
		s.mu.Unlock()
	}
	bbgo.Notify("✅ Strategy %s is up and running with %d candidate symbols: %v",
		s.InstanceID(),
		len(s.candidateSymbols),
		s.candidateSymbols,
	)

	// Register shutdown handler to persist state
	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		s.logger.Infof("shutting down %s", s.InstanceID())
		for symbol, executor := range s.spotGeneralOrderExecutors {
			if err := executor.GracefulCancel(ctx); err != nil {
				s.logger.WithError(err).Errorf("failed to gracefully cancel spot orders: %s", symbol)
			}
		}
		for symbol, executor := range s.futuresGeneralOrderExecutors {
			if err := executor.GracefulCancel(ctx); err != nil {
				s.logger.WithError(err).Errorf("failed to gracefully cancel futures orders: %s", symbol)
			}
		}
		// persist state
		bbgo.Sync(ctx, s)
		s.logger.Infof("state persisted for %s", s.InstanceID())
	})

	return nil
}

func (s *Strategy) allRounds() []*ArbitrageRound {
	var rounds []*ArbitrageRound
	for _, round := range s.ActiveRounds {
		rounds = append(rounds, round)
	}
	for _, pendingRound := range s.PendingRounds {
		rounds = append(rounds, pendingRound.Round)
	}
	for _, closedRound := range s.ClosedRoundTasks {
		rounds = append(rounds, closedRound.Round)
	}
	return rounds
}

func (s *Strategy) defaultBreaker(symbol string) *circuitbreaker.BasicCircuitBreaker {
	newBreaker := circuitbreaker.NewBasicCircuitBreaker(s.ID(), s.InstanceID(), symbol)
	newBreaker.MaximumConsecutiveLossTimes = 2
	newBreaker.HaltDuration = types.Duration(time.Hour * 16)
	return newBreaker
}

func (s *Strategy) tick(ctx context.Context, tickTime time.Time) {
	// lock the strategy to ensure all the updates to the active rounds are seen
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.lastTickTime.IsZero() && (s.lastTickTime.Equal(tickTime) || tickTime.Before(s.lastTickTime)) {
		return
	}
	s.lastTickTime = tickTime

	// start processing
	// 1. tick existing active rounds
	// We need to tick the active rounds first since we need to proceed the underlying workers to update the positions.
	// So that the following checks can accurately reflect the current status of the rounds.
	for _, round := range s.ActiveRounds {
		spotOrderBook := s.spotOrderBooks[round.SpotSymbol()].Copy()
		futuresOrderBook := s.futuresOrderBooks[round.FuturesSymbol()].Copy()
		oriState := round.State()
		round.Tick(ctx, tickTime, spotOrderBook, futuresOrderBook)
		currentState := round.State()
		if oriState != currentState {
			switch currentState {
			case RoundReady:
				bbgo.Notify("🟢 Round entered ready state: %s",
					round.SpotSymbol(),
					round.NewNotification(),
				)
			case RoundClosing:
				bbgo.Notify("🟡 Round is closing: %s",
					round.SpotSymbol(),
					round.NewNotification(),
				)
			case RoundClosed:
				bbgo.Notify("🔴 Round is closed: %s",
					round.SpotSymbol(),
					round.NewNotification(),
				)
			}
		}

	}
	// 2. transit active rounds
	for _, round := range s.ActiveRounds {
		posDeviation := round.CheckPositionDeviation(
			tickTime,
			s.CriticalErrorConfig.MaxMoqDeviation,
			time.Minute*15,
		)
		// record the round spot/futures positions
		roundPositionMetrics.With(
			prometheus.Labels{
				"strategy_id": s.InstanceID(),
				"symbol":      round.SpotSymbol(),
				"accountType": "spot",
			},
		).Set(posDeviation.SpotFilled.Float64())
		roundPositionMetrics.With(
			prometheus.Labels{
				"strategy_id": s.InstanceID(),
				"symbol":      round.FuturesSymbol(),
				"accountType": "futures",
			},
		).Set(posDeviation.FuturesFilled.Float64())
		roundQuantityDeviationMetrics.With(
			prometheus.Labels{
				"strategy_id": s.InstanceID(),
				"symbol":      round.SpotSymbol(),
			},
		).Set(posDeviation.DeviatedQuantity.Float64())

		if round.State() != RoundReady {
			// check if the round is halted or should be halted
			halted := round.IsHalted()
			roundSymbol := round.SpotSymbol()
			if !halted && posDeviation.DeviateTooLong {
				// the round is originally not halted but the deviation is too large -> we need to halt the round
				round.Halt(tickTime)
				s.logger.Warnf("round %s halted due to large hedge deviation: spot filled %s, futures filled %s (deviation %s)",
					roundSymbol,
					posDeviation.SpotFilled,
					posDeviation.FuturesFilled,
					posDeviation.DeviatedQuantity,
				)
				bbgo.Notify("💥 Round %s halted due to large hedge deviation. Manual intervention is required: spot filled %s, futures filled %s (deviation: %s)",
					roundSymbol,
					posDeviation.SpotFilled, posDeviation.FuturesFilled,
					posDeviation.DeviatedQuantity,
					round.NewCriticalNotification(),
				)
				continue
			} else if halted && !posDeviation.DeviateTooLong {
				// the deviation is back to normal, resume the round
				haltedAt := round.HaltedAt()
				round.Resume()
				s.logger.Infof("round %s resumed as hedge deviation back to normal: spot filled %s, futures filled %s",
					roundSymbol,
					posDeviation.SpotFilled,
					posDeviation.FuturesFilled,
				)
				bbgo.Notify("✅ Round %s resumed as hedge deviation back to normal. It was halted at %s.",
					roundSymbol,
					haltedAt.Format(time.RFC3339),
					round.NewNotification(),
				)
			}

			// round is still halted, skip the rest of the processing
			if round.IsHalted() {
				haltedAt := round.HaltedAt()
				elapsed := tickTime.Sub(haltedAt)
				limiter, found := s.haltNotificationLimiters[round.SpotSymbol()]
				if found && limiter.AllowN(tickTime, 1) {
					// send notification for rounds that have been halted for a while.
					bbgo.Notify("💥 Round %s halted for %s (since %s). Manual intervention is required",
						roundSymbol,
						elapsed.String(),
						haltedAt.Format(time.RFC3339),
						round.NewCriticalNotification(),
					)
				}
				continue
			}
		}

		s.transitRound(ctx, round, tickTime)

		// enque closed active rounds
		if round.State() == RoundClosed {
			s.logger.Infof("move round to closed queue: %s", round)
			// stop the round
			round.Stop()
			// remove from active round queue
			delete(s.ActiveRounds, round.SpotSymbol())
			// add to closed round queue for cleanup
			s.ClosedRoundTasks[round.SpotSymbol()] = &CloseRoundTask{
				Round:    round,
				RetryCnt: 0,
			}
		}
	}

	// 3. process closed round tasks
	closeRoundCtx, cancelCloseRound := context.WithTimeout(ctx, 15*time.Second)
	for _, task := range s.ClosedRoundTasks {
		round := task.Round
		if task.RetryCnt >= s.MaxClosedRetryCnt {
			if !task.Notified {
				// send notification for rounds that failed to pass the cleanup process for 3 times.
				// the symbol of the round will be blocked for opening new round until the issue is resolved.
				task.Notified = true
				bbgo.Notify("💥 Failed to handle closed round after %d retries. Manual intervention is required.",
					s.MaxClosedRetryCnt,
					round.NewCriticalNotification(),
				)
			}
			continue
		}

		task.LastTriedTime = tickTime
		task.RetryCnt++
		if err := s.handleClosedRound(closeRoundCtx, task, tickTime); err != nil {
			s.logger.WithError(err).Errorf("failed to handle closed round: %s", task.Round)
		} else {
			s.closedRoundStats(task.Round, tickTime)
			bbgo.Notify("✅ Successfully handled closed round: %s", round.String(), round.NewNotification())
			s.logger.Infof("successfully handled closed round: %s", task.Round)
			delete(s.ClosedRoundTasks, task.Round.SpotSymbol())
		}
	}
	cancelCloseRound()

	// 4. check if new round can be opened or existing round needs to be adjusted
	s.checkOpenNewRound(ctx, tickTime)

	// 5. process pending rounds that are waiting for fee asset preparation
	s.processPendingRounds(ctx, tickTime)

	// 6. log stats
	if s.lastStatsTime.IsZero() || tickTime.Sub(s.lastStatsTime) >= s.StatsPeriod.Duration() {
		s.notifyStats()
		s.lastStatsTime = tickTime
	}
}

func (s *Strategy) transitRound(ctx context.Context, round *ArbitrageRound, currentTime time.Time) {
	// still in the first funding interval, do not transit
	if round.NumHoldingIntervals(currentTime) <= 1 {
		if round.LastUpdateTime().IsZero() {
			round.SetUpdateTime(currentTime)
		}
		return
	}
	lastUpdateTime := round.LastUpdateTime()
	if lastUpdateTime.IsZero() {
		lastUpdateTime = currentTime
		round.SetUpdateTime(currentTime)
	}

	if currentTime.Sub(lastUpdateTime) < s.TransitRoundInterval.Duration() {
		return
	}

	oriState := round.State()
	switch oriState {
	case RoundOpening:
		s.transitOpeningOrReadyRoundToClosing(ctx, round, currentTime)
	case RoundReady:
		s.transitOpeningOrReadyRoundToClosing(ctx, round, currentTime)
	case RoundClosing:
		s.transitClosingRound(ctx, round, currentTime)
	}
	s.logger.Debugf("transit state for round %s: %s -> %s", round.SpotSymbol(), oriState, round.State())
	round.SetUpdateTime(currentTime)
}

func (s *Strategy) transitOpeningOrReadyRoundToClosing(ctx context.Context, round *ArbitrageRound, currentTime time.Time) {
	// if the current funding rate is still favorable, stay in current state, otherwise transit to closing
	timedCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	fundingRate, err := s.futuresService.QueryPremiumIndex(timedCtx, round.FuturesSymbol())
	if err != nil {
		return
	}

	// the funding rate has flipped
	if round.TriggeredFundingRate().Sign()*fundingRate.LastFundingRate.Sign() <= 0 {
		s.logger.Debugf("[transitOpeningOrReadyRound] funding rate flipped %s -> %s: %s",
			round.TriggeredFundingRate(), fundingRate.LastFundingRate, round)
		rateDiffAbs := fundingRate.LastFundingRate.Sub(round.TriggeredFundingRate()).Abs()
		if rateDiffAbs.Compare(s.CriticalErrorConfig.MaxFundingRateFlip) > 0 {
			bbgo.Notify("🚨 Round funding rate flip is too large: %s -> %s (threshold %s)",
				round.TriggeredFundingRate(),
				fundingRate.LastFundingRate,
				s.CriticalErrorConfig.MaxFundingRateFlip,
				round.NewCriticalNotification(),
			)
		}
		if round.NumHoldingIntervals(currentTime) >= round.MinHoldingIntervals() {
			// the min holding time has passed, transit to closing
			s.logger.Infof(
				"[transitOpeningOrReadyRound %s] min holding time passed, transit state %s -> closing, current funding rate %s: %s",
				currentTime.Format(time.RFC3339), round.State(), fundingRate.LastFundingRate, round)
			round.SetClosing(currentTime, s.TWAPWorkerConfig.ClosingDuration)
			return
		} else if currentTime.Sub(round.StartTime()) >= s.MarketSelectionConfig.MaxHoldingDuration.Duration() {
			s.logger.Infof(
				"[transitOpeningOrReadyRound %s] max holding hours reached, transit state %s -> closing, current funding rate %s: %s",
				currentTime.Format(time.RFC3339), round.State(), fundingRate.LastFundingRate, round,
			)
			round.SetClosing(currentTime, s.TWAPWorkerConfig.ClosingDuration)
			return
		}
		// the funding rate is not favorable anymore, check the exit cost to see if it's worth to transit to closing
		s.costEstimator.SetTargetPosition(round.TargetPosition())
		spotOrderBook, spotOk := s.spotOrderBooks[round.SpotSymbol()]
		futuresOrderBook, futuresOk := s.futuresOrderBooks[round.FuturesSymbol()]
		if !spotOk || !futuresOk {
			s.logger.Warnf("[transitOpeningOrReadyRound] order book not found for symbols: %s", round.SpotSymbol())
			return
		}
		cost, err := s.costEstimator.EstimateExitCost(true, spotOrderBook.Copy(), futuresOrderBook.Copy())
		if err != nil {
			s.logger.WithError(err).Errorf("[transitOpeningOrReadyRound] failed to estimate exit cost: %s", round.SpotSymbol())
			return
		}
		// the spread PnL is large enough to cover the exit cost, transit to closing
		if cost.SpreadPnL.Compare(cost.TotalFeeCost()) > 0 {
			s.logger.Infof(
				"[transitOpeningOrReadyRound %s] transit state %s -> closing, current funding rate %s: %s",
				currentTime.Format(time.RFC3339), round.State(), fundingRate.LastFundingRate, round)
			round.SetClosing(currentTime, s.TWAPWorkerConfig.ClosingDuration)
			return
		}
	}

	if s.allowLog(currentTime) {
		s.logger.Infof(
			"[transitOpeningOrReadyRound %s] round stays %s, current funding rate %s: %s",
			currentTime.Format(time.RFC3339), round.State(), fundingRate.LastFundingRate, round,
		)
	}
}

func (s *Strategy) transitClosingRound(ctx context.Context, round *ArbitrageRound, currentTime time.Time) {
	// the round is expired and is in closing state, keep it closing until it's closed
	if s.allowLog(currentTime) {
		s.logger.Infof("[transitClosingRound %s] round is closing: %s",
			currentTime.Format(time.RFC3339), round)
	}
}

func (s *Strategy) checkOpenNewRound(ctx context.Context, currentTime time.Time) {
	var lastOpenTime time.Time
	for _, round := range s.ActiveRounds {
		startTime := round.StartTime()
		if lastOpenTime.IsZero() {
			lastOpenTime = startTime
			continue
		}
		if startTime.After(lastOpenTime) {
			lastOpenTime = startTime
		}
	}
	if !lastOpenTime.IsZero() && currentTime.Sub(lastOpenTime) < s.OpenPositionInterval.Duration() {
		// still within the open position cooldown time, do not try to open new round
		return
	}

	// Only open new round when there is none
	// TODO: support multiple rounds for different symbols concurrently (e.g BTCUSDT and ETHUSDT)
	if len(s.allRounds()) == 0 {
		candidates, err := s.preliminaryMarketSelector.SelectMarkets(ctx, s.candidateSymbols)
		if err != nil {
			s.logger.WithError(err).Warn("failed to select market candidates")
			return
		}
		if len(candidates) == 0 {
			// no candidates, nothing to do
			return
		}
		s.logger.Debugf("candidates: %+v", candidates)

		var legitCandidates []MarketCandidate
		for _, candidate := range candidates {
			if s.canOpenRound(candidate.Symbol, currentTime) {
				legitCandidates = append(legitCandidates, candidate)
			}
		}
		s.logger.Debugf("legit candidates: %+v", legitCandidates)
		selectedCandidate := s.selectMostProfitableMarket(legitCandidates)
		if selectedCandidate == nil {
			// no profitable candidate found, nothing to do
			return
		}
		s.logger.Debugf("most profitable candidate for new round: %+v", selectedCandidate)

		// open new round if the estimated break-even holding interval is within the max holding hours
		if selectedCandidate.MinHoldingDuration <= s.MarketSelectionConfig.MaxHoldingDuration.Duration() {
			spotExecutor := s.spotGeneralOrderExecutors[selectedCandidate.Symbol]
			spotTwap, err := NewTWAPWorker(ctx, selectedCandidate.Symbol, s.spotSession, spotExecutor, s.TWAPWorkerConfig)
			spotTwap.SetLogger(s.logger)
			spotTwap.Executor().SetDryRun(s.DryRun)
			if err != nil || spotTwap == nil {
				s.logger.WithError(err).Errorf("failed to create TWAP worker for spot %s", selectedCandidate.Symbol)
				return
			}
			spotTwap.SetTargetPosition(selectedCandidate.TargetFuturesPosition.Neg())
			futuresExecutor := s.futuresGeneralOrderExecutors[selectedCandidate.Symbol]
			futuresTwap, err := NewTWAPWorker(ctx, selectedCandidate.Symbol, s.futuresSession, futuresExecutor, s.TWAPWorkerConfig)
			futuresTwap.SetLogger(s.logger)
			futuresTwap.Executor().SetDryRun(s.DryRun)
			if err != nil || futuresTwap == nil {
				s.logger.WithError(err).Errorf("failed to create TWAP worker for futures %s", selectedCandidate.Symbol)
				return
			}
			round := NewArbitrageRound(
				selectedCandidate.PremiumIndex,
				s.spotSession.Exchange.Name(),
				s.futuresSession.Exchange.Name(),
				selectedCandidate.MinHoldingIntervals,
				selectedCandidate.FundingIntervalHours,
				s.Leverage,
				spotTwap,
				futuresTwap,
				s.futuresService,
				s.MarketSelectionConfig.FuturesDirection,
			)
			round.SetLogger(s.logger)
			round.SetSpotExchangeFeeRates(
				s.costEstimator.GetSpotFeeRate(),
			)
			round.SetFuturesExchangeFeeRates(
				s.costEstimator.GetFuturesFeeRate(),
			)
			round.SetSlackAlert(s.SlackAlert)
			roundAnnualizedTriggerRateMetrics.With(
				prometheus.Labels{
					"strategy_id": s.InstanceID(),
					"symbol":      selectedCandidate.Symbol,
				},
			).Set(round.AnnualizedRate().Float64())
			// enqueue the new round to pending rounds for further processing
			s.PendingRounds[selectedCandidate.Symbol] = &PendingRound{
				Round: round,
			}
			bbgo.Notify("🆕 Created new pending round: %s", round.SpotSymbol())
		} else {
			s.logger.Debugf("selected candidate %s min holding duration too long: %s > %s, skipping",
				selectedCandidate.Symbol,
				selectedCandidate.MinHoldingDuration,
				s.MarketSelectionConfig.MaxHoldingDuration.Duration(),
			)
		}
	}
}

func (s *Strategy) allowLog(currentTime time.Time) bool {
	if s.logLimiter == nil {
		return false
	}
	return s.logLimiter.AllowN(currentTime, 1)
}

// static market filters
func (s *Strategy) filterMarketByCapSize(ctx context.Context, symbols []string) []string {
	if s.coinmarketcapClient == nil {
		return symbols
	}
	topAssets, err := s.queryTopCapAssets(ctx)
	if err != nil {
		return symbols
	}
	var candidateSymbols []string
	for _, symbol := range symbols {
		market, ok := s.futuresSession.Market(symbol)
		if !ok {
			continue
		}
		if _, found := topAssets[market.BaseCurrency]; found {
			candidateSymbols = append(candidateSymbols, symbol)
		}
	}
	return candidateSymbols
}

func (s *Strategy) filterMarketBothListed(symbols []string) []string {
	var candidateSymbols []string
	for _, symbol := range symbols {
		_, spotOk := s.spotSession.Market(symbol)
		_, futuresOk := s.futuresSession.Market(symbol)
		if spotOk && futuresOk {
			candidateSymbols = append(candidateSymbols, symbol)
		} else {
			s.logger.Infof("skipping %s as it's not listed on both spot and futures", symbol)
		}
	}
	return candidateSymbols
}

func (s *Strategy) filterMarketCollateralRate(ctx context.Context, symbols []string) []string {
	var markets []types.Market
	for _, symbol := range symbols {
		market, ok := s.futuresSession.Market(symbol)
		if !ok {
			continue
		}
		markets = append(markets, market)
	}
	var baseAssets []string
	for _, market := range markets {
		baseAssets = append(baseAssets, market.BaseCurrency)
	}
	collateralRates, err := queryPortfolioModeCollateralRates(ctx, baseAssets)
	if err != nil {
		s.logger.WithError(err).Warn("failed to query collateral rates, skipping collateral rate filter")
		return symbols
	}
	var candidateSymbols []string
	for _, market := range markets {
		rate, ok := collateralRates[market.BaseCurrency]
		if !ok {
			continue
		}
		if rate.Compare(s.MarketSelectionConfig.MinCollateralRate) >= 0 {
			candidateSymbols = append(candidateSymbols, market.Symbol)
		} else {
			s.logger.Infof("skipping %s due to low collateral rate: %s", market.Symbol, rate.String())
		}
	}
	return candidateSymbols
}

// selectMostProfitableMarket selects the most profitable market among the candidates based on the estimated break-even holding intervals
// it will also return the target position for the futures trade
// the most profitable market is the one with the shortest break-even holding intervals
func (s *Strategy) selectMostProfitableMarket(candidates []MarketCandidate) *MarketCandidate {
	if len(candidates) == 0 {
		return nil
	}
	spotAccount := s.spotSession.GetAccount()
	breakevenIntervals := make(map[string]fixedpoint.Value)
	targetFuturePositions := make(map[string]fixedpoint.Value)
	// totalQuoteAmount = price * targetSize * feeRateFactor, where feeRateFactor = 1 + 2*feeRate (buy and sell fee)
	// so the targetSize = totalQuoteAmount / (price * feeRateFactor)
	feeRate := s.costEstimator.GetSpotFeeRate().TakerFeeRate
	feeRateFactor := fixedpoint.One.Add(feeRate.Mul(fixedpoint.Two))
	for _, candidate := range candidates {
		spotMarket, spotOk := s.spotSession.Market(candidate.Symbol)
		futuresMarket, futuresOk := s.futuresSession.Market(candidate.Symbol)
		if !spotOk || !futuresOk {
			continue
		}
		var targetSize, minHoldingIntervals fixedpoint.Value
		var negPosition bool
		if s.MarketSelectionConfig.FuturesDirection == types.PositionShort {
			negPosition = true
			if candidate.PremiumIndex.LastFundingRate.Sign() <= 0 {
				// the funding rate is not favorable for short futures, skip
				continue
			}
			// long spot -> find the amount for the quote currency
			quoteBalance, ok := spotAccount.Balance(spotMarket.QuoteCurrency)
			if !ok {
				continue
			}
			totalQuoteAmount := quoteBalance.Available.Mul(s.MarketSelectionConfig.TradeBalanceRatio)
			// long spot -> trade on the sell side of the order book
			// targetSize = totalQuoteAmount / (price * feeRateFactor)
			sellBook := s.spotOrderBooks[candidate.Symbol].SideBook(types.SideTypeSell)
			spotPrice := sellBook.AverageDepthPriceByQuote(totalQuoteAmount, 0)
			targetSize = totalQuoteAmount.Div(spotPrice.Mul(feeRateFactor))
			// short futures -> trade on the buy side of the order book
			buyBook := s.futuresOrderBooks[candidate.Symbol].SideBook(types.SideTypeBuy)
			futuresPrice := buyBook.AverageDepthPrice(targetSize)
			// short futures -> target future position should be negative
			holdingIntervals, err := s.calculateMinHoldingIntervals(candidate, futuresPrice, targetSize.Neg())
			if err != nil {
				continue
			}
			minHoldingIntervals = holdingIntervals
		} else if s.MarketSelectionConfig.FuturesDirection == types.PositionLong {
			negPosition = false
			if candidate.PremiumIndex.LastFundingRate.Sign() >= 0 {
				// the funding rate is not favorable for long futures, skip
				continue
			}
			baseBalance, ok := spotAccount.Balance(spotMarket.BaseCurrency)
			if !ok {
				continue
			}
			// totalQuoteAmount = price * totalBase and targetSize = totalQuoteAmount / (price * feeRateFactor)
			// so targetSize = totalBase / feeRateFactor
			totalBase := baseBalance.Available.Mul(s.MarketSelectionConfig.TradeBalanceRatio)
			targetSize = totalBase.Div(feeRateFactor)
			// long futures -> trade on the sell side of the order book
			sellBook := s.futuresOrderBooks[candidate.Symbol].SideBook(types.SideTypeSell)
			futuresPrice := sellBook.AverageDepthPrice(targetSize)
			// long futures -> target future position should be positive
			holdingIntervals, err := s.calculateMinHoldingIntervals(candidate, futuresPrice, totalBase)
			if err != nil {
				continue
			}
			minHoldingIntervals = holdingIntervals
		} else {
			return nil
		}
		targetSize = fixedpoint.Min(
			spotMarket.TruncateQuantity(targetSize),
			futuresMarket.TruncateQuantity(targetSize),
		)
		breakevenIntervals[candidate.Symbol] = minHoldingIntervals
		if negPosition {
			targetSize = targetSize.Neg()
		}
		targetFuturePositions[candidate.Symbol] = targetSize
	}
	if len(breakevenIntervals) == 0 {
		return nil
	}
	// shallow copy the candidates slice
	sortedCandidates := append([]MarketCandidate(nil), candidates...)
	sort.Slice(sortedCandidates, func(i, j int) bool {
		candidate1 := sortedCandidates[i]
		candidate2 := sortedCandidates[j]
		return breakevenIntervals[candidate1.Symbol].Compare(breakevenIntervals[candidate2.Symbol]) <= 0
	})
	bestCandidate := &sortedCandidates[0]
	targetPosition := targetFuturePositions[bestCandidate.Symbol]
	if targetPosition.IsZero() {
		return nil
	}

	bestMarket, ok := s.spotSession.Market(bestCandidate.Symbol)
	if !ok {
		return nil
	}
	if maxExposure, ok := s.MaxPositionExposure[bestMarket.BaseCurrency]; ok && targetPosition.Abs().Compare(maxExposure) > 0 {
		if targetPosition.Sign() > 0 {
			targetPosition = maxExposure
		} else {
			targetPosition = maxExposure.Neg()
		}
	}
	// set the estimated min holding interval for the selected candidate
	bestCandidate.MinHoldingIntervals = breakevenIntervals[bestCandidate.Symbol].Int()
	numHoldingHours := bestCandidate.MinHoldingIntervals * bestCandidate.FundingIntervalHours
	bestCandidate.MinHoldingDuration = time.Duration(numHoldingHours) * time.Hour
	bestCandidate.TargetFuturesPosition = targetPosition
	return bestCandidate
}

func (s *Strategy) calculateMinHoldingIntervals(candidate MarketCandidate, bestPrice, targetPosition fixedpoint.Value) (fixedpoint.Value, error) {
	s.costEstimator.SetTargetPosition(targetPosition)
	spotOrderBook, spotFound := s.spotOrderBooks[candidate.Symbol]
	futuresOrderBook, futuresFound := s.futuresOrderBooks[candidate.Symbol]
	if !spotFound || !futuresFound {
		return fixedpoint.Zero, errors.New("order book not found for candidate symbol")
	}
	spotOrderBookSnapshot := spotOrderBook.Copy()
	futuresOrderBookSnapshot := futuresOrderBook.Copy()

	estimateEntryCost, err := s.costEstimator.EstimateEntryCost(true, spotOrderBookSnapshot, futuresOrderBookSnapshot)
	if err != nil {
		return fixedpoint.Zero, err
	}
	estimateExitCost, err := s.costEstimator.EstimateExitCost(true, spotOrderBookSnapshot, futuresOrderBookSnapshot)
	if err != nil {
		return fixedpoint.Zero, err
	}
	totalCost := estimateEntryCost.
		TotalFeeCost().
		Add(estimateEntryCost.SpreadPnL).
		Add(estimateExitCost.TotalFeeCost()).
		Add(estimateExitCost.SpreadPnL)
	amount := targetPosition.Abs().Mul(bestPrice)
	estimateFundingFeePerInterval := amount.Mul(candidate.PremiumIndex.LastFundingRate.Abs())
	if estimateFundingFeePerInterval.IsZero() {
		return fixedpoint.Zero, fmt.Errorf("estimated funding fee per interval is zero for candidate %s", candidate.Symbol)
	}
	breakEvenIntervals := totalCost.Div(estimateFundingFeePerInterval).Round(0, fixedpoint.Up)
	return breakEvenIntervals, nil
}

type CloseRoundTask struct {
	Round         *ArbitrageRound `json:"round"`
	RetryCnt      int             `json:"retry_cnt"`
	LastTriedTime time.Time       `json:"last_tried_time"`
	Notified      bool            `json:"notified"`
}

// handleClosedRound handles the cleanup of a closed round
// the returned error of this function will be considered as a critical error
func (s *Strategy) handleClosedRound(ctx context.Context, task *CloseRoundTask, tickTime time.Time) error {
	round := task.Round
	futuresOrderbook := s.futuresOrderBooks[round.FuturesSymbol()].Copy()
	futuresMidPrice := getMidPrice(futuresOrderbook)
	// if the remaining quantity is too large, return an critical error
	// the error will prevent the round being removed from closed rounds and stop creating new round on the same symbol
	remainingNotional := round.FuturesWorker().RemainingQuantity().Mul(futuresMidPrice)
	if remainingNotional.Abs().Compare(s.CriticalErrorConfig.MaxRemainingNotional) >= 0 {
		return fmt.Errorf(
			"[handleClosedRound] remaining notional %s > %s: %s",
			remainingNotional,
			s.CriticalErrorConfig.MaxRemainingNotional,
			round,
		)
	}

	// clean up open orders if there is any
	if err := round.SpotWorker().Executor().CancelOpenOrders(ctx); err != nil {
		return fmt.Errorf(
			"[handleClosedRound] failed to cancel open spot orders for %s: %w",
			round.SpotSymbol(),
			err,
		)
	}
	if err := round.FuturesWorker().Executor().CancelOpenOrders(ctx); err != nil {
		return fmt.Errorf(
			"[handleClosedRound] failed to cancel open futures orders for %s: %w",
			round.FuturesSymbol(),
			err,
		)
	}

	// close futures positions if there is any
	if err := round.Cleanup(ctx, futuresOrderbook); err != nil {
		return fmt.Errorf("[handleClosedRound] failed to close remaining positions for the futures: %w", err)
	}
	// remove the orders from the order store
	spotExecutor := s.spotGeneralOrderExecutors[round.SpotSymbol()]
	for _, order := range round.SpotWorker().Executor().AllOrders() {
		spotExecutor.OrderStore().Remove(order)
	}
	futuresExecutor := s.futuresGeneralOrderExecutors[round.FuturesSymbol()]
	for _, order := range round.FuturesWorker().Executor().AllOrders() {
		futuresExecutor.OrderStore().Remove(order)
	}

	// transfer any residual collateral back to the spot account.
	asset := round.CollateralAsset()
	account, err := s.futuresSession.UpdateAccount(ctx)
	if err != nil {
		return fmt.Errorf("[handleClosedRound] failed to update futures account when handling round exit: %w", err)
	}
	// get balance
	balance, ok := account.Balance(asset)
	if !ok {
		return fmt.Errorf("[handleClosedRound] balance not found for asset %s when handling round exit: %s", asset, round)
	}
	// compute the amount to transfer back to spot account
	residualAmount := s.computeResidualCollateral(task, balance, asset)

	// transfer the collateral back to spot account when the available balance is sufficient
	if residualAmount.Sign() > 0 {
		if err := s.futuresService.TransferFuturesAccountAsset(ctx, asset, residualAmount, types.TransferOut); err != nil {
			return fmt.Errorf("[handleClosedRound] failed to transfer %s %s during round exit: %w", balance.Available, asset, err)
		} else {
			bbgo.Notify("⬅️ Transferred %s %s back to spot account",
				balance.Available,
				asset,
				round.NewNotification(),
			)
		}
	}

	// set fee average cost
	if executor, ok := s.spotGeneralOrderExecutors[s.FeeSymbol]; ok {
		feeAvgCost := executor.Position().AverageCost
		round.SetAvgFeeCost(s.FeeSymbol, feeAvgCost)
	}
	// sync funding fee records for the round
	if err := round.SyncFundingFeeRecords(ctx, tickTime); err != nil {
		return fmt.Errorf("[handleClosedRound] failed to sync funding fee records for round %s: %w", round, err)
	}
	return nil
}

func (s *Strategy) closedRoundStats(round *ArbitrageRound, tickTime time.Time) {
	pnl := round.PnL()
	bbgo.Notify("Round PnL %s", round.SpotSymbol(), pnl)
	if breaker, found := s.CircuitBreakers[round.SpotSymbol()]; found {
		breaker.RecordProfit(pnl.NetPnL(), tickTime)
	} else {
		s.logger.Warnf("circuit breaker not found for symbol %s when recording profit: %s", round.SpotSymbol(), pnl)
	}
	labels := prometheus.Labels{
		"strategy_id": s.InstanceID(),
		"symbol":      round.SpotSymbol(),
	}
	roundHoldingIntervalMetrics.With(labels).Set(
		float64(round.NumHoldingIntervals(tickTime)),
	)
	roundNetPnLMetrics.With(labels).Set(pnl.NetPnL().Float64())
	// TODO: insert closed round records into database
}

func (s *Strategy) computeResidualCollateral(task *CloseRoundTask, balance types.Balance, asset string) fixedpoint.Value {
	amount := fixedpoint.Zero
	var closingSide types.SideType
	if task.Round.TriggeredTargetPosition().Sign() > 0 {
		// short futures
		// closing trade side is buy
		closingSide = types.SideTypeBuy
	} else {
		// long futures
		// closing trade side is sell
		closingSide = types.SideTypeSell
	}

	for _, trade := range task.Round.FuturesWorker().Executor().AllTrades() {
		if trade.Side != closingSide {
			continue
		}
		amount = amount.Add(task.Round.syncState.DirectionPolicy.TransferAmountFromFuturesTrade(trade))
	}
	diffAmount := amount.Sub(task.Round.syncState.TransferOutAmount)
	return fixedpoint.Min(diffAmount, balance.Net())
}

func (s *Strategy) canOpenRound(symbol string, currentTime time.Time) bool {
	// cannot open new round for the symbol if it's halted
	if breaker, found := s.CircuitBreakers[symbol]; found {
		if reason, halted := breaker.IsHalted(currentTime); halted {
			s.logger.Warnf("[canOpenRound] circuit breaker is triggered for symbol %s: %s", symbol, reason)
			return false
		}
	}

	// can only open new round for the symbol if there is none
	_, pending := s.PendingRounds[symbol]
	_, active := s.ActiveRounds[symbol]
	_, closed := s.ClosedRoundTasks[symbol]

	return !pending && !active && !closed
}

func (s *Strategy) newDebugLogger() *logrus.Entry {
	logger := logrus.New()
	logger.SetFormatter(logrus.StandardLogger().Formatter)
	logger.SetOutput(logrus.StandardLogger().Out)
	logger.SetLevel(logrus.DebugLevel)
	return logger.WithFields(logrus.Fields{
		"strategy":    ID,
		"strategy_id": s.InstanceID(),
	})
}

func (s *Strategy) notifyStats() {
	var activeRoundNotifications []any
	for _, round := range s.ActiveRounds {
		activeRoundNotifications = append(activeRoundNotifications, round.NewNotification())
	}

	var pendingRoundNotifications []any
	for _, pendingRound := range s.PendingRounds {
		pendingRoundNotifications = append(pendingRoundNotifications, pendingRound.Round.NewNotification())
	}

	bbgo.Notify("📊 Round stats: %d active rounds, %d pending rounds",
		len(s.ActiveRounds),
		len(s.PendingRounds),
	)
	if len(activeRoundNotifications) > 0 {
		bbgo.Notify("Active Rounds", activeRoundNotifications...)
	}
	if len(pendingRoundNotifications) > 0 {
		bbgo.Notify("Pending Rounds", pendingRoundNotifications...)
	}

}

// Fee rate and BNB discount.
// See: https://www.binance.com/en/fee/futureFee
func (s *Strategy) setupBinance(ctx context.Context, session *bbgo.ExchangeSession) error {
	binanceEx, ok := session.Exchange.(*binance.Exchange)
	if !ok {
		s.logger.Infof("[setupBinance] session %s is not a binance exchange, skipping", session.Name)
		return nil
	}

	enabled, err := binanceEx.QueryBnbBurnStatus(ctx)
	if err != nil {
		return fmt.Errorf("[setupBinance] failed to query BNB burn status for %s: %w", session.Name, err)
	}
	if enabled {
		s.logger.Infof("[setupBinance] BNB burn is enabled for %s", session.Name)
		if s.FeeSymbol == "" {
			s.logger.Infof("[setupBinance] overriding fee symbol to %s", s.FeeSymbol)
			s.FeeSymbol = "BNB" + s.QuoteCurrency
		}
		accountType := "spot"
		if binanceEx.IsFutures {
			accountType = "futures"
		}
		if _, found := s.FeeDiscountRate[accountType]; !found {
			var defaultRate fixedpoint.Value
			switch accountType {
			case "spot":
				defaultRate = fixedpoint.NewFromFloat(0.25)
			case "futures":
				defaultRate = fixedpoint.NewFromFloat(0.1)
			}
			s.logger.Infof("[setupBinance] overriding %s fee discount rate to %s", accountType, defaultRate.Percentage())
			s.FeeDiscountRate[accountType] = defaultRate
		}
	}
	return nil
}

func (s *Strategy) checkAndFixMarginMode(ctx context.Context) error {
	binanceFutures, ok := s.futuresSession.Exchange.(*binance.Exchange)
	if !ok {
		s.logger.Infof("[checkAndFixMarginMode] futures session %s is not a binance exchange, skipping", s.futuresSession.Name)
		return nil
	}
	futuresClient := binanceFutures.GetFuturesClient()
	req := futuresClient.NewFuturesGetMultiAssetsModeRequest()
	resp, err := req.Do(ctx)
	if err != nil {
		return err
	}

	if resp.MultiAssetsMargin {
		return nil
	}

	fixReq := futuresClient.NewFuturesChangeMultiAssetsModeRequest()
	fixReq.MultiAssetsMargin(binanceapi.MultiAssetsMarginModeOn)
	fixResp, err := fixReq.Do(ctx)
	if err != nil {
		return err
	}

	s.logger.Infof("changeMultiAssetsMode response: %+v", fixResp)
	return nil
}

func (s *Strategy) prepareRounds(ctx context.Context) error {
	for symbol, round := range s.ActiveRounds {
		err := round.Prepare(ctx, s.spotSession, s.futuresSession)
		if err != nil {
			return fmt.Errorf("fail to prepare round %s: %w", symbol, err)
		}
	}
	return nil
}

func (s *Strategy) setLeverage(ctx context.Context) error {
	for _, symbol := range s.candidateSymbols {
		if err := s.futuresService.SetLeverage(ctx, symbol, s.Leverage); err != nil {
			return fmt.Errorf("failed to set leverage for symbol %s: %w", symbol, err)
		}
	}
	return nil
}

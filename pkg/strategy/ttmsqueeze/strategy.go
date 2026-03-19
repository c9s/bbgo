package ttmsqueeze

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "mtf-ttmsqueeze"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

// Strategy implements the MTF TTM Squeeze Strategy
// Entry conditions (Long):
// 1. HTF bullish: HTF close > HTF 21 EMA
// 2. Squeeze just fired: was squeezing (compression), now released (no compression)
// 3. Momentum > 0 and increasing (MomentumDirectionBullish)
// 4. Price above LTF 21 EMA
//
// Exit conditions:
// - Price below LTF 21 EMA
// - Unrealized loss exceeds MaxLossRatio
// - Momentum weakening (BullishSlowing)
// Exit is done via TWAP over 2 LTF intervals with NumExitOrders orders
type Strategy struct {
	*common.Strategy

	Environment *bbgo.Environment
	Market      types.Market

	// Symbol is the market symbol
	Symbol string `json:"symbol"`

	// LtfInterval is the lower time frame (LTF) interval for signal generation
	LtfInterval types.Interval `json:"ltfInterval"`

	// HtfInterval is the higher time frame (HTF) interval for trend definition
	HtfInterval types.Interval `json:"htfInterval"`

	// Window is the squeeze length (default: 20)
	Window int `json:"window"`

	// EMAWindow is the EMA length for trend (default: 21)
	EMAWindow int `json:"emaWindow"`

	// EntryConfig contains entry-related settings (required)
	EntryConfig *EntryConfig `json:"entry"`

	// ExitConfig contains exit-related settings
	ExitConfig *ExitConfig `json:"exit,omitempty"`

	// TwapConfig contains TWAP execution settings
	TwapConfig *TwapConfig `json:"twap,omitempty"`

	// internal fields
	session *bbgo.ExchangeSession

	// indicators
	ltfTTMSqueeze *indicatorv2.TTMSqueezeStream
	ltfEMA        *indicatorv2.EWMAStream
	htfEMA        *indicatorv2.EWMAStream
	htfClose      *indicatorv2.PriceStream

	// state tracking
	prevCompressionLevel indicatorv2.CompressionLevel

	// consecutive signal counters
	consecutiveEntrySignals int // counts consecutive entry signals
	consecutiveExitSignals  int // counts consecutive momentum weakening signals

	// state machine handles state transitions and TWAP execution
	stateMachine *StateMachine

	statsWorker *StatsWorker

	// hard exit cooldown tracking
	hardExitTime time.Time
	hardExitMu   sync.RWMutex

	// position recovery
	recoveryInProgress atomic.Bool

	logger *logrus.Entry
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s:%s", ID, s.Symbol, s.LtfInterval)
}

func (s *Strategy) Defaults() error {
	if s.Window == 0 {
		s.Window = 20
	}

	if s.EMAWindow == 0 {
		s.EMAWindow = 21
	}

	if s.EntryConfig == nil {
		s.EntryConfig = &EntryConfig{}
	}
	s.EntryConfig.Defaults()

	if s.ExitConfig == nil {
		s.ExitConfig = &ExitConfig{}
	}
	s.ExitConfig.Defaults()

	if s.TwapConfig == nil {
		s.TwapConfig = &TwapConfig{}
	}
	s.TwapConfig.Defaults()

	return nil
}

func (s *Strategy) Validate() error {
	if s.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}

	if s.LtfInterval == "" {
		return fmt.Errorf("LtfInterval is required")
	}
	if s.HtfInterval == "" {
		return fmt.Errorf("HtfInterval is required")
	}
	if s.LtfInterval.Duration() > s.HtfInterval.Duration() {
		return fmt.Errorf("LtfInterval must be lower than HtfInterval")
	}

	if s.EntryConfig == nil {
		return fmt.Errorf("entry config is required")
	}
	if err := s.EntryConfig.Validate(); err != nil {
		return err
	}

	return nil
}

func (s *Strategy) Initialize() error {
	if s.Strategy == nil {
		s.Strategy = &common.Strategy{}
	}
	s.logger = logrus.WithFields(
		logrus.Fields{
			"strategy": s.ID(),
			"symbol":   s.Symbol,
		})
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.LtfInterval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.HtfInterval})
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.session = session
	market, found := session.Market(s.Symbol)
	if !found {
		return fmt.Errorf("market not found for %s on %s", s.Symbol, session.Name)
	}
	s.Market = market

	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, ID, s.InstanceID())

	// Clean up any open orders from previous runs
	if err := s.cancelAllOpenOrders(ctx); err != nil {
		s.logger.WithError(err).Warn("failed to cleanup open orders on startup")
	}

	// Initialize state machine
	s.stateMachine = NewStateMachine(
		session.Exchange,
		s.Symbol,
		s.Market,
		s.TwapConfig,
		s.Strategy.Position,
		s.logger,
	)
	s.stateMachine.CheckAndUpdateState()

	// Handle state changes
	s.stateMachine.OnStateChange(func(from, to State) {
		s.logger.Infof("strategy state changed: %s -> %s", from, to)
		if to == StateIdle {
			// Reset counters when returning to Idle
			s.consecutiveEntrySignals = 0
			s.consecutiveExitSignals = 0
		}
	})

	// keep track of position and profit on trade updates
	session.UserDataStream.OnTradeUpdate(types.TradeWith(
		s.Symbol,
		func(trade types.Trade) {
			profit, netProfit, madeProfit := s.Position.AddTrade(trade)
			if !madeProfit {
				return
			}
			p := s.Position.NewProfit(trade, profit, netProfit)
			s.ProfitStats.AddTrade(trade)
			s.ProfitStats.AddProfit(p)
			s.Environment.RecordPosition(s.Position, trade, &p)

			s.statsWorker.BasePositionC <- s.Position.GetBase()

			// Check if position is closed and update state
			s.stateMachine.CheckAndUpdateState()

			bbgo.Sync(ctx, s)
		}),
	)

	// Backfill historical klines BEFORE creating indicators
	// so that KLineStream.BackFill() has data to work with
	if err := s.backfillKlines(ctx); err != nil {
		return fmt.Errorf("failed to backfill klines: %w", err)
	}

	// Setup LTF indicators (will automatically backfill from store)
	indicators := session.Indicators(s.Symbol)

	s.ltfTTMSqueeze = indicators.TTMSqueeze(types.IntervalWindow{
		Interval: s.LtfInterval,
		Window:   s.Window,
	})
	s.ltfEMA = indicators.EWMA(types.IntervalWindow{
		Interval: s.LtfInterval,
		Window:   s.EMAWindow,
	})

	// Setup HTF indicators
	s.htfEMA = indicators.EWMA(types.IntervalWindow{
		Interval: s.HtfInterval,
		Window:   s.EMAWindow,
	})
	s.htfClose = indicators.CLOSE(s.HtfInterval)

	// Initialize previous compression level
	s.prevCompressionLevel = indicatorv2.CompressionLevelNone

	var workerWg sync.WaitGroup
	workerCtx, workerCancel := context.WithCancel(ctx)

	// Start position monitor to prevent overshooting
	go s.monitorWorker(&workerWg, workerCtx)

	// Start risk management worker for hard exits
	go s.riskWorker(&workerWg, workerCtx)

	// Initialize state worker
	s.statsWorker = NewRecordWorker(s.stateMachine.GetState())
	s.statsWorker.Run(&workerWg, workerCtx)
	s.stateMachine.OnStateChange(func(from, to State) {
		// Record state transition
		now := time.Now()
		record := StateRecord{
			State:     from,
			NextState: to,
			EndTime:   now,
		}
		select {
		case s.statsWorker.RecordC <- &record:
		default:
			s.logger.Warnf("state record channel full, dropping record: %s (%s)", from, record.Duration())
		}
	})

	// Handle TTM Squeeze updates
	s.ltfTTMSqueeze.OnUpdate(func(squeeze indicatorv2.TTMSqueeze) {
		s.handleSqueezeUpdate(ctx, squeeze)
	})

	// Graceful shutdown
	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		s.stateMachine.Shutdown(ctx)
		workerCancel()

		s.statsWorker.Stop()

		if err := s.Strategy.OrderExecutor.GracefulCancel(ctx); err != nil {
			s.logger.WithError(err).Error("failed to gracefully cancel orders")
		}

		// Clear position with market orders if configured
		if s.ExitConfig.ClearPositionOnShutdown {
			if err := s.clearPositionOnShutdown(ctx); err != nil {
				s.logger.WithError(err).Error("failed to clear position on shutdown")
			}
		}

		// Secondary check: query and cancel any remaining open orders via REST API
		if err := s.cancelAllOpenOrders(ctx); err != nil {
			s.logger.WithError(err).Warn("failed to cancel open orders on shutdown")
		}

		bbgo.Sync(ctx, s)
	})

	return nil
}

func (s *Strategy) handleSqueezeUpdate(ctx context.Context, squeeze indicatorv2.TTMSqueeze) {
	// Get current values
	ltfEMAValue := s.ltfEMA.Last(0)
	htfEMAValue := s.htfEMA.Last(0)
	htfCloseValue := s.htfClose.Last(0)

	// Get current price
	currentPrice, ok := s.session.LastPrice(s.Symbol)
	if !ok {
		s.logger.Warn("failed to get last price")
		return
	}

	// Check HTF bullish condition: HTF close > HTF 21 EMA
	htfBullish := htfCloseValue > htfEMAValue

	// Check squeeze fired: was compressing (any level > None), now released (None)
	squeezeFired := s.prevCompressionLevel != indicatorv2.CompressionLevelNone &&
		squeeze.CompressionLevel == indicatorv2.CompressionLevelNone

	// Check momentum increasing: mom > 0 and mom > prev (MomentumDirectionBullish)
	momIncreasing := squeeze.MomentumDirection == indicatorv2.MomentumDirectionBullish

	// Check price above LTF EMA
	priceAboveEMA := currentPrice.Float64() > ltfEMAValue

	// Update previous compression level for next iteration
	s.prevCompressionLevel = squeeze.CompressionLevel

	// Check position state
	position := s.Strategy.Position
	hasPosition := !position.IsDust(currentPrice)

	// Get current state machine state
	currentState := s.stateMachine.GetState()

	// Check exit conditions if we have a position (max loss is handled by riskWorker)
	if hasPosition && position.IsLong() && currentState != StateExiting {
		// Exit 1: Price below LTF 21 EMA
		priceBelowEMA := currentPrice.Float64() < ltfEMAValue
		if priceBelowEMA {
			s.logger.Infof("exit: price below EMA, closing position")
			s.triggerExit(ctx)
			return
		}

		// Exit 2: Momentum weakening (BullishSlowing) - requires consecutive signals
		if currentState == StateLong {
			momWeakening := squeeze.MomentumDirection == indicatorv2.MomentumDirectionBullishSlowing
			if momWeakening {
				s.consecutiveExitSignals++
				if s.consecutiveExitSignals >= s.ExitConfig.ConsecutiveExit {
					s.logger.Infof("exit: momentum weakening (%d consecutive signals), closing position",
						s.consecutiveExitSignals)
					s.triggerExit(ctx)
					return
				}
			} else if squeeze.MomentumDirection == indicatorv2.MomentumDirectionBullish {
				// Reset counter when momentum strengthens
				s.consecutiveExitSignals = 0
			}
		}
	}

	// Check entry conditions
	longCondition := htfBullish && squeezeFired && momIncreasing && priceAboveEMA

	if longCondition {
		s.consecutiveEntrySignals++
		if s.consecutiveEntrySignals >= s.EntryConfig.ConsecutiveEntries {
			// Check if we are in hard exit cooldown
			if s.isInCooldown() {
				s.logger.Infof("in hard exit cooldown, skipping entry")
				return
			}

			// Check max position limit
			if s.exceedsMaxPosition() {
				s.logger.Infof("max position quantity reached, skipping entry")
				return
			}

			// Don't enter if there is currently a buy TWAP executing
			if s.stateMachine.IsTwapRunning() && s.stateMachine.GetTwapSide() == types.SideTypeBuy {
				s.logger.Infof("buy TWAP execution in progress, skipping entry")
				return
			}

			if currentState == StateExiting {
				s.logger.Infof("re-entry signal while exiting: HTF bullish=%v, squeeze fired=%v, momentum increasing=%v, price above EMA=%v (%d consecutive signals)",
					htfBullish, squeezeFired, momIncreasing, priceAboveEMA, s.consecutiveEntrySignals)
			} else {
				s.logger.Infof("entry signal: HTF bullish=%v, squeeze fired=%v, momentum increasing=%v, price above EMA=%v (%d consecutive signals)",
					htfBullish, squeezeFired, momIncreasing, priceAboveEMA, s.consecutiveEntrySignals)
			}

			s.openLongPosition(ctx)
			// Reset counter after entry
			s.consecutiveEntrySignals = 0
		}
	} else {
		// Reset counter when conditions are not met
		s.consecutiveEntrySignals = 0
	}
}

func (s *Strategy) exceedsMaxPosition() bool {
	// With position monitor, base should never be negative
	// If it is, we shouldn't be entering anyway
	currentBase := s.Strategy.Position.GetBase()
	if currentBase.Sign() <= 0 {
		return false
	}
	return currentBase.Compare(s.EntryConfig.MaxPosition) >= 0
}

// setHardExitTime records the time of a hard exit for cooldown tracking
func (s *Strategy) setHardExitTime(now time.Time) {
	s.hardExitMu.Lock()
	defer s.hardExitMu.Unlock()
	s.hardExitTime = now
}

// isInCooldown checks if we are still in the hard exit cooldown period
func (s *Strategy) isInCooldown() bool {
	s.hardExitMu.RLock()
	defer s.hardExitMu.RUnlock()
	if s.hardExitTime.IsZero() {
		return false
	}
	cooldownEnd := s.hardExitTime.Add(s.ExitConfig.HardExitCoolDown.Duration())
	return time.Now().Before(cooldownEnd)
}

// cancelAllOpenOrders queries open orders via REST API and cancels them
func (s *Strategy) cancelAllOpenOrders(ctx context.Context) error {
	timedCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	openOrders, err := s.session.Exchange.QueryOpenOrders(timedCtx, s.Symbol)
	if err != nil {
		return fmt.Errorf("failed to query open orders: %w", err)
	}

	if len(openOrders) == 0 {
		s.logger.Debug("no open orders found for cancellation")
		return nil
	}

	s.logger.Warnf("found %d remaining open orders, cancelling...", len(openOrders))

	if err := s.session.Exchange.CancelOrders(timedCtx, openOrders...); err != nil {
		return fmt.Errorf("failed to cancel open orders: %w", err)
	}

	s.logger.Infof("successfully cancelled %d open orders", len(openOrders))
	return nil
}

// clearPositionOnShutdown closes the position with multiple market orders
func (s *Strategy) clearPositionOnShutdown(ctx context.Context) error {
	base := s.Strategy.Position.GetBase()
	if base.IsZero() {
		s.logger.Info("no position to clear on shutdown")
		return nil
	}

	quantity := base.Abs()

	// dust check
	isDust := false
	if currentPrice, ok := s.session.LastPrice(s.Symbol); ok {
		isDust = s.Position.IsDust(currentPrice)
	} else {
		isDust = s.Position.IsDust()
	}

	if isDust {
		s.logger.Info("shutdown position is dust, skipping")
		return nil
	}

	numOrders, sliceQuantity := s.calculateOrderParams(quantity, s.ExitConfig.NumOrders)

	s.logger.Infof("clearing position on shutdown with %d market orders, total=%s, slice=%s",
		numOrders, quantity.String(), sliceQuantity.String())

	// Submit market sell orders
	remaining := quantity
	var side types.SideType = types.SideTypeSell
	if s.Position.IsShort() {
		side = types.SideTypeBuy
	}

	var lastErr error
	for i := 0; i < numOrders && remaining.Sign() > 0; i++ {
		orderQty := sliceQuantity
		// Last order takes the remainder
		if i == numOrders-1 || remaining.Compare(sliceQuantity) <= 0 {
			orderQty = s.Market.TruncateQuantity(remaining)
		}

		if orderQty.Compare(s.Market.MinQuantity) < 0 {
			break
		}

		order := types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     side,
			Type:     types.OrderTypeMarket,
			Quantity: orderQty,
		}

		_, err := s.Strategy.OrderExecutor.SubmitOrders(ctx, order)
		if err != nil {
			s.logger.WithError(err).Errorf("failed to submit shutdown market order %d/%d", i+1, numOrders)
			lastErr = err
		} else {
			s.logger.Infof("shutdown order %d/%d submitted: %s", i+1, numOrders, orderQty.String())
		}

		remaining = remaining.Sub(orderQty)
	}

	return lastErr
}

// backfillKlines queries historical klines and adds them to the MarketDataStore
// to initialize indicators before live trading starts
func (s *Strategy) backfillKlines(ctx context.Context) error {
	store, ok := s.session.MarketDataStore(s.Symbol)
	if !ok {
		return fmt.Errorf("market data store not found for %s", s.Symbol)
	}

	// Backfill LTF klines
	if err := s.backfillInterval(ctx, store, s.LtfInterval); err != nil {
		return fmt.Errorf("failed to backfill LTF klines: %w", err)
	}

	// Backfill HTF klines
	if err := s.backfillInterval(ctx, store, s.HtfInterval); err != nil {
		return fmt.Errorf("failed to backfill HTF klines: %w", err)
	}

	return nil
}

func (s *Strategy) backfillInterval(ctx context.Context, store *types.MarketDataStore, interval types.Interval) error {
	// Query enough klines for indicators (at least Window * 2 for safety)
	limit := s.Window * 2
	if limit < 100 {
		limit = 100
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	endTime := time.Now()
	klines, err := s.session.Exchange.QueryKLines(queryCtx, s.Symbol, interval, types.KLineQueryOptions{
		EndTime: &endTime,
		Limit:   limit,
	})
	if err != nil {
		return err
	}

	if len(klines) == 0 {
		return fmt.Errorf("no klines returned for backfill %s", interval)
	}

	for _, k := range klines {
		store.AddKLine(k)
	}

	s.logger.Infof("backfilled %d klines for %s %s", len(klines), s.Symbol, interval)
	return nil
}

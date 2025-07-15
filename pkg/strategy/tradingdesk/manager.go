package tradingdesk

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type TradingManagerMap map[string]*TradingManager

func (m TradingManagerMap) Get(
	ctx context.Context, environ *bbgo.Environment, session *bbgo.ExchangeSession, market types.Market,
	strategy *Strategy,
) *TradingManager {
	manager, ok := m[market.Symbol]
	if ok {
		return manager
	}

	manager = m.New(ctx, environ, session, market, strategy)
	m[market.Symbol] = manager
	return manager
}

func (m TradingManagerMap) New(
	ctx context.Context, environ *bbgo.Environment, session *bbgo.ExchangeSession, market types.Market,
	strategy *Strategy,
) *TradingManager {
	manager := &TradingManager{}
	manager.Initialize(ctx, environ, session, market, strategy)
	return manager
}

type TradingManagerState struct {
	Position         *types.Position    `json:"position,omitempty"`
	ProfitStats      *types.ProfitStats `json:"profitStats,omitempty"`
	TakeProfitOrders types.OrderSlice   `json:"takeProfitOrders,omitempty"`
	StopLossOrders   types.OrderSlice   `json:"stopLossOrders,omitempty"`

	ExpiryTime *time.Time `json:"expiryTime,omitempty"`
}

type TradingManager struct {
	TradingManagerState

	orderExecutor *bbgo.GeneralOrderExecutor

	// References to strategy components
	session  *bbgo.ExchangeSession
	market   types.Market
	strategy *Strategy

	logger logrus.FieldLogger

	closeTrigger       func()
	cancelCloseTrigger context.CancelFunc
	closePositionMutex sync.Mutex
}

func (m *TradingManager) Initialize(
	ctx context.Context,
	environ *bbgo.Environment, session *bbgo.ExchangeSession, market types.Market,
	strategy *Strategy,
) {
	strategyID := strategy.ID()
	instanceID := strategy.InstanceID()

	m.session = session
	m.market = market
	m.strategy = strategy
	m.logger = strategy.logger.WithField("symbol", market.Symbol)

	if m.Position == nil {
		m.Position = types.NewPositionFromMarket(market)
		m.Position.Strategy = strategyID
		m.Position.StrategyInstanceID = instanceID

		m.Position.SetExchangeFeeRate(session.ExchangeName, types.ExchangeFee{
			MakerFeeRate: session.MakerFeeRate,
			TakerFeeRate: session.TakerFeeRate,
		})
	}

	if m.ProfitStats == nil {
		m.ProfitStats = types.NewProfitStats(market)
	}

	m.orderExecutor = bbgo.NewGeneralOrderExecutor(session, market.Symbol, strategyID, instanceID, m.Position)
	m.orderExecutor.BindEnvironment(environ)
	m.orderExecutor.BindProfitStats(m.ProfitStats)
	m.orderExecutor.Bind()
	m.orderExecutor.ActiveMakerOrders().OnFilled(m.OnOrderFilled)

	if m.ExpiryTime != nil {
		m.closeTrigger, m.cancelCloseTrigger = m.createCloseTrigger(ctx, time.Until(*m.ExpiryTime))
		go m.closeTrigger()
	} else {
		m.closeTrigger = nil
		m.cancelCloseTrigger = nil
	}
}

func (m *TradingManager) OnOrderFilled(order types.Order) {
	matched := false
	if len(m.TakeProfitOrders) > 0 {
		if _, ok := m.TakeProfitOrders.FindByOrderID(order.OrderID); ok {
			matched = true
		}
	}

	if len(m.StopLossOrders) > 0 {
		if _, ok := m.StopLossOrders.FindByOrderID(order.OrderID); ok {
			if matched {
				m.logger.Warnf("order %d matched both take profit and stop loss orders, this is unexpected", order.OrderID)
			}

			matched = true
		}
	}

	if matched && m.cancelCloseTrigger != nil {
		m.cancelCloseTrigger()
	}
}

func (m *TradingManager) SetLeverage(ctx context.Context, lv int) error {
	return m.strategy.riskService.SetLeverage(ctx, m.market.Symbol, lv)
}

// OpenPosition opens a new position with risk-based position sizing.
// The position size is calculated based on MaxLossLimit, stop loss price, and available balance.
func (m *TradingManager) OpenPosition(ctx context.Context, params OpenPositionParams) error {
	base := m.Position.GetBase()
	if !base.IsZero() {
		return fmt.Errorf("position already exists for %s: %s", params.Symbol, base.String())
	}

	switch strings.ToUpper(string(params.Side)) {
	case "LONG":
		params.Side = types.SideTypeBuy
	case "SHORT":
		params.Side = types.SideTypeSell
	}

	// get current price for validation
	ticker, err := m.session.Exchange.QueryTicker(ctx, params.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get ticker for %s: %w", params.Symbol, err)
	}

	ticker.GetValidPrice()

	currentPrice := ticker.GetPrice(params.Side, m.strategy.PriceType)
	if currentPrice.IsZero() {
		return fmt.Errorf("invalid current price %s for %s", currentPrice.String(), params.Symbol)
	}

	ok, err := validateStopLossTakeProfit(params.Side, currentPrice, params.StopLossPrice, params.TakeProfitPrice)
	if !ok {
		m.logger.Warnf("invalid SL or TP price setup, fallback to kline calculation: %v", err)

		stopLoss, takeProfit, ferr := fallbackStopLossTakeProfit(ctx, m.session.Exchange, params.Symbol, time.Now(), 8*time.Hour, params.Side, 0.05)
		if ferr != nil {
			return fmt.Errorf("fallback kline calculation failed: %w", ferr)
		}

		params.StopLossPrice = stopLoss
		params.TakeProfitPrice = takeProfit
		m.logger.Infof("fallback SL price: %s, TP profit price: %s", stopLoss.String(), takeProfit.String())

		// check again
		ok, err = validateStopLossTakeProfit(params.Side, currentPrice, params.StopLossPrice, params.TakeProfitPrice)
		if !ok {
			return fmt.Errorf("stop loss or take profit price still invalid after fallback: %w", err)
		}
	}

	// Calculate position size based on risk management
	quantity, err := m.calculatePositionSize(ctx, params)
	if err != nil {
		return err
	}

	order := types.SubmitOrder{
		Symbol:   params.Symbol,
		Side:     params.Side,
		Type:     types.OrderTypeMarket,
		Market:   m.market,
		Quantity: quantity,
	}

	createdOrders, err := m.orderExecutor.SubmitOrders(ctx, order)
	if err != nil {
		return fmt.Errorf("failed to open position for %s: %w", params.Symbol, err)
	}

	m.logger.Infof("created orders: %+v", createdOrders)

	if params.TimeToLive > 0 {
		expiryTime := time.Now().Add(params.TimeToLive.Duration())
		m.ExpiryTime = &expiryTime

		m.closeTrigger, m.cancelCloseTrigger = m.createCloseTrigger(ctx, params.TimeToLive.Duration())
		go m.closeTrigger()
	}

	if params.StopLossPrice.Sign() > 0 {
		stopLossOrders, err := m.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
			Market:        m.market,
			Symbol:        params.Symbol,
			Side:          params.Side.Reverse(),
			Type:          types.OrderTypeStopMarket,
			StopPrice:     params.StopLossPrice,
			ClosePosition: true,
		})

		if err != nil {
			return fmt.Errorf("failed to submit stop loss orders for %s: %w", params.Symbol, err)
		}

		m.strategy.logger.Infof("created stop loss orders: %+v", stopLossOrders)
		m.StopLossOrders = stopLossOrders
	}

	if params.TakeProfitPrice.Sign() > 0 {
		orderType := types.OrderTypeStopMarket

		if m.session.Futures && m.session.ExchangeName == types.ExchangeBinance {
			orderType = types.OrderTypeTakeProfitMarket
		}

		takeProfitOrders, err := m.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
			Market:        m.market,
			Symbol:        params.Symbol,
			Side:          params.Side.Reverse(),
			Type:          orderType,
			StopPrice:     params.TakeProfitPrice,
			ClosePosition: true,
		})

		if err != nil {
			return fmt.Errorf("failed to submit take profit orders for %s: %w", params.Symbol, err)
		}

		m.strategy.logger.Infof("created take loss orders: %+v", takeProfitOrders)
		m.TakeProfitOrders = takeProfitOrders
	}

	m.logger.Infof("opened position: %s", m.Position.String())
	return nil
}

func (m *TradingManager) createCloseTrigger(baseCtx context.Context, ttl time.Duration) (func(), context.CancelFunc) {
	ctx, cancel := context.WithCancel(baseCtx)

	m.logger.Infof("creating close trigger with ttl %s for position %s", ttl.String(), m.Position.String())
	return func() {
		select {
		case <-ctx.Done():
			return

		case <-time.AfterFunc(ttl, func() {
			m.logger.Infof("position expired (%s), closing position", ttl)

			if err := m.ClosePosition(baseCtx); err != nil {
				m.logger.WithError(err).Error("failed to close position")
			}

		}).C:
			return
		}
	}, cancel
}

func (m *TradingManager) GetPosition() *types.Position {
	return m.Position
}

func (m *TradingManager) ClosePosition(ctx context.Context) error {
	m.closePositionMutex.Lock()
	defer m.closePositionMutex.Unlock()

	base := m.Position.GetBase()
	if m.Position == nil || base.IsZero() {
		return nil
	}

	// Close position by submitting a market order in the opposite direction
	order := types.SubmitOrder{
		Symbol:     m.market.Symbol,
		Side:       m.Position.Side().Reverse(),
		Type:       types.OrderTypeMarket,
		Market:     m.market,
		Quantity:   base.Abs(),
		ReduceOnly: true,

		// ClosePosition is only used for stop market
		ClosePosition: false,
	}

	_, err := m.orderExecutor.SubmitOrders(ctx, order)
	if err != nil {
		return fmt.Errorf("failed to close position for %s order: %+v: %w", m.market.Symbol, order, err)
	}

	if m.cancelCloseTrigger != nil {
		m.cancelCloseTrigger()
	}

	// reset expiry time
	m.TradingManagerState.ExpiryTime = nil

	if err := m.orderExecutor.CancelOrders(ctx, m.TakeProfitOrders...); err != nil {
		m.logger.WithError(err).Warnf("failed to cancel orders")
	}

	if err := m.orderExecutor.CancelOrders(ctx, m.StopLossOrders...); err != nil {
		m.logger.WithError(err).Warnf("failed to cancel orders")
	}

	// reset orders
	m.TakeProfitOrders = nil
	m.StopLossOrders = nil
	return nil
}

// calculatePositionSize calculates the position size based on risk management and available balance.
// The function performs the following steps:
// 1. Returns original quantity if no stop loss is provided
// 2. Fetches current market price from exchange
// 3. Calculates risk per unit based on stop loss distance
// 4. Computes maximum quantity allowed by risk limit (MaxLossLimit / riskPerUnit)
// 5. Determines available balance constraint:
//   - Futures mode: uses quote currency balance / current price
//   - Spot buy orders: uses quote currency balance / current price
//   - Spot sell orders: uses base currency balance
//
// 6. Returns the minimum of risk-limited quantity and balance-limited quantity
//
// This ensures positions never exceed risk tolerance or available funds.
func (m *TradingManager) calculatePositionSize(
	ctx context.Context, param OpenPositionParams,
) (fixedpoint.Value, error) {
	// Check if stop loss is provided, if not return original quantity
	if param.StopLossPrice.IsZero() {
		return param.Quantity, nil
	}

	// Get current market price
	ticker, err := m.session.Exchange.QueryTicker(ctx, param.Symbol)
	if err != nil {
		return fixedpoint.Zero, fmt.Errorf("failed to get ticker for %s: %w", param.Symbol, err)
	}

	currentPrice := m.strategy.PriceType.GetPrice(ticker, param.Side)
	if currentPrice.IsZero() {
		return fixedpoint.Zero, fmt.Errorf("invalid current price for %s", param.Symbol)
	}

	// stopLossRange returns the price range to the stop loss
	riskPerUnit := stopLossRange(param.Side, currentPrice, param.StopLossPrice)
	if riskPerUnit.Sign() <= 0 {
		return fixedpoint.Zero, createInvalidStopLossError(param.Side, currentPrice)
	}

	maxQuantityByRisk := m.strategy.MaxLossLimit.Div(riskPerUnit)

	account := m.session.GetAccount()
	if m.session.Futures {
		quoteBalance, ok := account.Balance(m.market.QuoteCurrency)
		if !ok {
			return fixedpoint.Zero, fmt.Errorf("no balance found for %s", m.market.QuoteCurrency)
		}

		maxPositionSize := quoteBalance.Available.Mul(fixedpoint.NewFromInt(int64(m.strategy.Leverage))).Div(currentPrice)
		maxPositionSize = m.market.TruncateQuantity(maxPositionSize)
		maxPositionSize = fixedpoint.Min(maxPositionSize, maxQuantityByRisk)

		maxPositionSize = fixedpoint.Max(maxPositionSize, m.market.MinQuantity)
		maxPositionSize = m.market.AdjustQuantityByMinNotional(maxPositionSize, currentPrice)

		m.logger.Infof("max position size %s by quote balance: %s, current price: %s, leverage: %d", maxPositionSize.String(), quoteBalance.String(), currentPrice.String(), m.strategy.Leverage)
		return maxPositionSize, nil
	} else {
		if param.Side == types.SideTypeBuy {
			quoteBalance, ok := account.Balance(m.market.QuoteCurrency)
			if !ok {
				return fixedpoint.Zero, fmt.Errorf("no balance found for %s", m.market.QuoteCurrency)
			}
			return fixedpoint.Min(quoteBalance.Available.Div(currentPrice), maxQuantityByRisk), nil
		}

		baseBalance, ok := account.Balance(m.market.BaseCurrency)
		if !ok {
			return fixedpoint.Zero, fmt.Errorf("no balance found for %s", m.market.BaseCurrency)
		}
		return fixedpoint.Min(baseBalance.Available, maxQuantityByRisk), nil
	}
}

// stopLossRange calculates the risk per unit based on current price, stop loss, and side
func stopLossRange(side types.SideType, currentPrice, stopLossPrice fixedpoint.Value) fixedpoint.Value {
	if side == types.SideTypeBuy {
		// For long positions, risk is current price - stop loss price
		return currentPrice.Sub(stopLossPrice)
	}
	// For short positions, risk is stop loss price - current price
	return stopLossPrice.Sub(currentPrice)
}

// createInvalidStopLossError creates an appropriate error message for invalid stop loss prices
func createInvalidStopLossError(side types.SideType, currentPrice fixedpoint.Value) error {
	if side == types.SideTypeBuy {
		return fmt.Errorf("invalid stop loss price for buy order: stop loss should be below current price (%s)", currentPrice.String())
	}
	return fmt.Errorf("invalid stop loss price for sell order: stop loss should be above current price (%s)", currentPrice.String())
}

// validateStopLossTakeProfit checks the validity of stop loss and take profit prices based on side and current price.
func validateStopLossTakeProfit(
	side types.SideType, currentPrice, stopLossPrice, takeProfitPrice fixedpoint.Value,
) (bool, error) {
	switch side {
	case types.SideTypeBuy:
		if stopLossPrice.Sign() > 0 && stopLossPrice.Compare(currentPrice) > 0 {
			return false, fmt.Errorf("stop loss price (%s) must be less than current price (%s) for long position", stopLossPrice.String(), currentPrice.String())
		}

		if takeProfitPrice.Sign() > 0 && takeProfitPrice.Compare(currentPrice) < 0 {
			return false, fmt.Errorf("take profit price (%s) must be greater than current price (%s) for long position", takeProfitPrice.String(), currentPrice.String())
		}

	case types.SideTypeSell:
		if stopLossPrice.Sign() > 0 && stopLossPrice.Compare(currentPrice) < 0 {
			return false, fmt.Errorf("stop loss price must be greater than current price for short position")
		}

		if takeProfitPrice.Sign() > 0 && takeProfitPrice.Compare(currentPrice) > 0 {
			return false, fmt.Errorf("take profit price must be less than current price for short position")
		}

	default:
		return false, fmt.Errorf("invalid side type: %s", side)
	}

	return true, nil
}

// fallbackStopLossTakeProfit calculates fallback stop loss and take profit prices from kline data.
// percent is the adjustment ratio for take profit if the last kline is the extreme point (e.g. 0.05 for 5%).
// duration is the lookback window for kline analysis (e.g. 8 * time.Hour).
func fallbackStopLossTakeProfit(
	ctx context.Context, exchange types.Exchange, symbol string, now time.Time, duration time.Duration,
	side types.SideType, percent float64,
) (stopLoss, takeProfit fixedpoint.Value, err error) {
	interval := types.Interval15m
	endTime := now
	startTime := now.Add(-duration)
	klines, err := exchange.QueryKLines(ctx, symbol, interval, types.KLineQueryOptions{
		StartTime: &startTime,
		EndTime:   &endTime,
	})

	if err != nil || len(klines) == 0 {
		return fixedpoint.Zero, fixedpoint.Zero, fmt.Errorf("failed to query klines, error: %w", err)
	}

	minLow := klines[0].Low
	maxHigh := klines[0].High
	minIdx := 0
	maxIdx := 0
	for i, k := range klines {
		if k.Low.Compare(minLow) <= 0 {
			minLow = k.Low
			minIdx = i
		}
		if k.High.Compare(maxHigh) >= 0 {
			maxHigh = k.High
			maxIdx = i
		}
	}
	last := klines[len(klines)-1]
	adj := fixedpoint.NewFromFloat(1 + percent)
	adjDown := fixedpoint.NewFromFloat(1 - percent)

	switch side {
	case types.SideTypeBuy:
		stopLoss = minLow
		if maxIdx == len(klines)-1 {
			takeProfit = last.High.Mul(adj)
		} else {
			takeProfit = maxHigh
		}
	case types.SideTypeSell:
		stopLoss = maxHigh
		if minIdx == len(klines)-1 {
			takeProfit = last.Low.Mul(adjDown)
		} else {
			takeProfit = minLow
		}
	}
	return stopLoss, takeProfit, nil
}

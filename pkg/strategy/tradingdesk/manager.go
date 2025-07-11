package tradingdesk

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type TradingManagerState struct {
	Position    *types.Position    `json:"position,omitempty"`
	ProfitStats *types.ProfitStats `json:"profitStats,omitempty"`

	TakeProfitOrders []types.Order `json:"takeProfitOrders,omitempty"`
	StopLossOrders   []types.Order `json:"stopLossOrders,omitempty"`
}

type TradingManager struct {
	TradingManagerState

	orderExecutor *bbgo.GeneralOrderExecutor

	// References to strategy components
	session  *bbgo.ExchangeSession
	market   types.Market
	strategy *Strategy

	logger logrus.FieldLogger
}

func (m *TradingManager) Initialize(
	ctx context.Context, environ *bbgo.Environment, session *bbgo.ExchangeSession, market types.Market,
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
}

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

	// Calculate position size based on risk management
	quantity, err := m.calculatePositionSize(ctx, params)
	if err != nil {
		return err
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
		m.logger.Errorf("invalid stop loss or take profit price: %v", err)
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

func (m *TradingManager) GetPosition() *types.Position {
	return m.Position
}

func (m *TradingManager) ClosePosition(ctx context.Context) error {
	if m.Position == nil {
		return fmt.Errorf("no position to close for %s", m.market.Symbol)
	}

	// Close position by submitting a market order in the opposite direction
	order := types.SubmitOrder{
		Symbol:     m.market.Symbol,
		Side:       m.Position.Side().Reverse(),
		Type:       types.OrderTypeMarket,
		Market:     m.market,
		Quantity:   m.Position.GetBase().Abs(),
		ReduceOnly: true,

		// ClosePosition is only used for stop market
		ClosePosition: false,
	}

	_, err := m.orderExecutor.SubmitOrders(ctx, order)
	if err != nil {
		return fmt.Errorf("failed to close position for %s order: %+v: %w", m.market.Symbol, order, err)
	}

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
	if side == types.SideTypeBuy {
		if stopLossPrice.Sign() > 0 && stopLossPrice >= currentPrice {
			return false, fmt.Errorf("stop loss price must be less than current price for long position")
		}
		if takeProfitPrice.Sign() > 0 && takeProfitPrice <= currentPrice {
			return false, fmt.Errorf("take profit price must be greater than current price for long position")
		}
	} else if side == types.SideTypeSell {
		if stopLossPrice.Sign() > 0 && stopLossPrice <= currentPrice {
			return false, fmt.Errorf("stop loss price must be greater than current price for short position")
		}
		if takeProfitPrice.Sign() > 0 && takeProfitPrice >= currentPrice {
			return false, fmt.Errorf("take profit price must be less than current price for short position")
		}
	}
	return true, nil
}

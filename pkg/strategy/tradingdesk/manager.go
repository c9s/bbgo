package tradingdesk

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type TradingManager struct {
	Position      *types.Position    `persistence:"position"`
	ProfitStats   *types.ProfitStats `persistence:"profitStats"`
	OrderExecutor *bbgo.GeneralOrderExecutor

	// Configuration for position opening
	MaxLossLimit fixedpoint.Value
	PriceType    types.PriceType

	// References to strategy components
	Session *bbgo.ExchangeSession
	Market  types.Market
}

func (m *TradingManager) Initialize(ctx context.Context, environ *bbgo.Environment, session *bbgo.ExchangeSession, market types.Market, strategyID, instanceID string) {
	if m.Position == nil {
		m.Position = types.NewPositionFromMarket(market)
		m.Position.Strategy = strategyID
		m.Position.StrategyInstanceID = instanceID
	}

	if m.ProfitStats == nil {
		m.ProfitStats = types.NewProfitStats(market)
	}

	m.OrderExecutor = bbgo.NewGeneralOrderExecutor(session, market.Symbol, strategyID, instanceID, m.Position)
	m.OrderExecutor.BindEnvironment(environ)
	m.OrderExecutor.BindProfitStats(m.ProfitStats)
	m.OrderExecutor.Bind()

	// Store references for position opening
	m.Session = session
	m.Market = market

	// Set default price type if not set
	if m.PriceType == "" {
		m.PriceType = types.PriceTypeMaker
	}
}

type TradingManagerMap map[string]*TradingManager

func (m TradingManagerMap) GetTradingManager(ctx context.Context, environ *bbgo.Environment, session *bbgo.ExchangeSession, symbol, strategyID, instanceID string) (*TradingManager, error) {
	market, ok := session.Market(symbol)
	if !ok {
		return nil, fmt.Errorf("market %s not found", symbol)
	}

	manager, ok := m[symbol]
	if !ok {
		manager = &TradingManager{}
		m[symbol] = manager
	}

	manager.Initialize(ctx, environ, session, market, strategyID, instanceID)
	return manager, nil
}

// OpenPosition opens a new position with risk-based position sizing.
// The position size is calculated based on MaxLossLimit, stop loss price, and available balance.
func (m *TradingManager) OpenPosition(ctx context.Context, param OpenPositionParam) error {
	// Calculate position size based on risk management
	quantity, err := m.calculatePositionSize(ctx, param)
	if err != nil {
		return err
	}

	order := types.SubmitOrder{
		Symbol:    param.Symbol,
		Side:      param.Side,
		Type:      types.OrderTypeMarket,
		Quantity:  quantity,
		StopPrice: param.StopPrice,
	}

	createdOrders, err := m.OrderExecutor.SubmitOrders(ctx, order)
	if err != nil {
		logrus.WithError(err).Errorf("failed to submit market order: %+v", order)
		return err
	}
	logrus.Infof("created orders: %+v", createdOrders)
	return nil
}

// calculatePositionSize calculates the optimal position size based on risk management parameters.
// It considers three factors to determine the final quantity:
// 1. MaxLossLimit: Limits maximum potential loss per position in quote currency
// 2. Available balance: Ensures sufficient funds for the trade
// 3. Original quantity: User-specified desired quantity
//
// The function uses stop loss price to calculate risk per unit:
// - For buy orders: risk = currentPrice - stopLossPrice
// - For sell orders: risk = stopLossPrice - currentPrice
//
// Final quantity = min(originalQuantity, maxQuantityByRisk, maxQuantityByBalance)
// where maxQuantityByRisk = MaxLossLimit / riskPerUnit
//
// Example: If MaxLossLimit=100 USDT, currentPrice=50000, stopLoss=49000
// then riskPerUnit=1000 USDT, maxQuantityByRisk=0.1 BTC
func (m *TradingManager) calculatePositionSize(ctx context.Context, param OpenPositionParam) (fixedpoint.Value, error) {
	// Check if stop loss is provided, if not return original quantity
	if param.StopPrice.IsZero() {
		return param.Quantity, nil
	}

	// Get current market price
	ticker, err := m.Session.Exchange.QueryTicker(ctx, param.Symbol)
	if err != nil {
		return fixedpoint.Zero, fmt.Errorf("failed to get ticker for %s: %w", param.Symbol, err)
	}

	currentPrice := m.PriceType.GetPrice(ticker, param.Side)
	if currentPrice.IsZero() {
		return fixedpoint.Zero, fmt.Errorf("invalid current price for %s", param.Symbol)
	}

	riskPerUnit := m.stopLossRange(currentPrice, param.StopPrice, param.Side)
	if riskPerUnit.Sign() <= 0 {
		return fixedpoint.Zero, createInvalidStopLossError(param.Side, currentPrice)
	}

	availableBalance, err := m.getAvailableBalance(param.Side)
	if err != nil {
		return fixedpoint.Zero, err
	}

	// Calculate maximum quantity based on MaxLossLimit
	var maxQuantityByRisk fixedpoint.Value
	if !m.MaxLossLimit.IsZero() {
		maxQuantityByRisk = m.MaxLossLimit.Div(riskPerUnit)
	} else {
		maxQuantityByRisk = param.Quantity
	}

	// Calculate maximum quantity based on available balance
	var maxQuantityByBalance fixedpoint.Value
	if param.Side == types.SideTypeBuy {
		// For buy orders: available quote currency / price
		maxQuantityByBalance = availableBalance.Div(currentPrice)
	} else {
		// For sell orders: available base currency
		maxQuantityByBalance = availableBalance
	}

	// Use the minimum of the three: original quantity, risk-based limit, balance-based limit
	quantity := fixedpoint.Min(param.Quantity, fixedpoint.Min(maxQuantityByRisk, maxQuantityByBalance))

	// Ensure quantity is positive
	if quantity.Sign() <= 0 {
		return fixedpoint.Zero, fmt.Errorf("calculated quantity is zero or negative")
	}

	logrus.Infof("Position size calculation: symbol=%s, currentPrice=%s, stopLoss=%s, riskPerUnit=%s, maxLossLimit=%s, availableBalance=%s, finalQuantity=%s",
		param.Symbol, currentPrice, param.StopPrice, riskPerUnit, m.MaxLossLimit, availableBalance, quantity)

	return quantity, nil
}

// stopLossRange calculates the risk per unit based on current price, stop loss, and side
func (m *TradingManager) stopLossRange(currentPrice, stopLossPrice fixedpoint.Value, side types.SideType) fixedpoint.Value {
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

// getAvailableBalance returns the available balance for the appropriate currency based on order side
func (m *TradingManager) getAvailableBalance(side types.SideType) (fixedpoint.Value, error) {
	account := m.Session.GetAccount()

	var currency string
	if side == types.SideTypeBuy {
		currency = m.Market.QuoteCurrency // For buy orders, we need quote currency balance
	} else {
		currency = m.Market.BaseCurrency // For sell orders, we need base currency balance
	}

	balance, ok := account.Balance(currency)
	if !ok {
		return fixedpoint.Zero, fmt.Errorf("no balance found for %s", currency)
	}

	return balance.Available, nil
}

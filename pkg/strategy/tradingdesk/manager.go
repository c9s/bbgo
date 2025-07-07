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
		StopPrice: param.StopLossPrice,
	}

	createdOrders, err := m.OrderExecutor.SubmitOrders(ctx, order)
	if err != nil {
		logrus.WithError(err).Errorf("failed to submit market order: %+v", order)
		return err
	}
	logrus.Infof("created orders: %+v", createdOrders)
	return nil
}

// calculatePositionSize calculates the position size based on risk management and available balance.
// The function performs the following steps:
// 1. Returns original quantity if no stop loss is provided
// 2. Fetches current market price from exchange
// 3. Calculates risk per unit based on stop loss distance
// 4. Computes maximum quantity allowed by risk limit (MaxLossLimit / riskPerUnit)
// 5. Determines available balance constraint:
//    - Futures mode: uses quote currency balance / current price
//    - Spot buy orders: uses quote currency balance / current price  
//    - Spot sell orders: uses base currency balance
// 6. Returns the minimum of risk-limited quantity and balance-limited quantity
//
// This ensures positions never exceed risk tolerance or available funds.
func (m *TradingManager) calculatePositionSize(ctx context.Context, param OpenPositionParam) (fixedpoint.Value, error) {
	// Check if stop loss is provided, if not return original quantity
	if param.StopLossPrice.IsZero() {
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

	riskPerUnit := m.stopLossRange(currentPrice, param.StopLossPrice, param.Side)
	if riskPerUnit.Sign() <= 0 {
		return fixedpoint.Zero, createInvalidStopLossError(param.Side, currentPrice)
	}

	maxQuantityByRisk := m.MaxLossLimit.Div(riskPerUnit)
	account := m.Session.GetAccount()
	if m.Session.Futures {
		quoteBalance, ok := account.Balance(m.Market.QuoteCurrency)
		if !ok {
			return fixedpoint.Zero, fmt.Errorf("no balance found for %s", m.Market.QuoteCurrency)
		}
		return fixedpoint.Min(quoteBalance.Available.Div(currentPrice), maxQuantityByRisk), nil
	} else {
		if param.Side == types.SideTypeBuy {
			quoteBalance, ok := account.Balance(m.Market.QuoteCurrency)
			if !ok {
				return fixedpoint.Zero, fmt.Errorf("no balance found for %s", m.Market.QuoteCurrency)
			}
			return fixedpoint.Min(quoteBalance.Available.Div(currentPrice), maxQuantityByRisk), nil
		}

		baseBalance, ok := account.Balance(m.Market.BaseCurrency)
		if !ok {
			return fixedpoint.Zero, fmt.Errorf("no balance found for %s", m.Market.BaseCurrency)
		}
		return fixedpoint.Min(baseBalance.Available, maxQuantityByRisk), nil
	}
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

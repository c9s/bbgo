package tradingdesk

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "tradingdesk"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type PositionMap map[string]*types.Position
type ProfitStatsMap map[string]*types.ProfitStats

type Strategy struct {
	Session     *bbgo.ExchangeSession
	Environment *bbgo.Environment

	OrderExecutorMap map[string]*bbgo.GeneralOrderExecutor
	PositionMap      PositionMap    `persistence:"position_map"`
	ProfitStatsMap   ProfitStatsMap `persistence:"profit_stats_map"`

	MaxLossLimit fixedpoint.Value `json:"maxLossLimit"`
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return ID
}

func (s *Strategy) Initialize() error {
	if s.PositionMap == nil {
		s.PositionMap = make(PositionMap)
	}

	if s.ProfitStatsMap == nil {
		s.ProfitStatsMap = make(ProfitStatsMap)
	}

	if s.OrderExecutorMap == nil {
		s.OrderExecutorMap = make(map[string]*bbgo.GeneralOrderExecutor)
	}
	return nil
}

func (s *Strategy) Validate() error {
	return nil
}

// Subscribe implements the strategy interface for market data subscriptions
func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// Currently no market data subscriptions needed
}

// Run implements the strategy interface for strategy execution
func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.Session = session
	// Strategy is ready to receive OpenPosition calls
	return nil
}

func (s *Strategy) getOrCreatePosition(symbol string) (*types.Position, error) {
	market, ok := s.Session.Market(symbol)
	if !ok {
		return nil, fmt.Errorf("market %s not found", symbol)
	}

	if position, ok := s.PositionMap[symbol]; ok {
		return position, nil
	}

	position := types.NewPositionFromMarket(market)
	position.Strategy = ID
	position.StrategyInstanceID = s.InstanceID()
	s.PositionMap[symbol] = position

	return position, nil
}

func (s *Strategy) getOrCreateProfitStats(symbol string) (*types.ProfitStats, error) {
	market, ok := s.Session.Market(symbol)
	if !ok {
		return nil, fmt.Errorf("market %s not found", symbol)
	}

	if profitStats, ok := s.ProfitStatsMap[symbol]; ok {
		return profitStats, nil
	}

	profitStats := types.NewProfitStats(market)
	s.ProfitStatsMap[symbol] = profitStats

	return profitStats, nil
}

func (s *Strategy) getOrCreateOrderExecutor(symbol string) (*bbgo.GeneralOrderExecutor, error) {
	if executor, ok := s.OrderExecutorMap[symbol]; ok {
		return executor, nil
	}

	position, err := s.getOrCreatePosition(symbol)
	if err != nil {
		return nil, err
	}

	profitStats, err := s.getOrCreateProfitStats(symbol)
	if err != nil {
		return nil, err
	}

	executor := bbgo.NewGeneralOrderExecutor(s.Session, symbol, s.ID(), s.InstanceID(), position)
	executor.BindEnvironment(s.Environment)
	executor.BindProfitStats(profitStats)
	executor.Bind()

	s.OrderExecutorMap[symbol] = executor
	return executor, nil
}

// OpenPosition opens a new position with risk-based position sizing.
// The position size is calculated based on MaxLossLimit, stop loss price, and available balance.
func (s *Strategy) OpenPosition(ctx context.Context, param OpenPositionParam) error {
	executor, err := s.getOrCreateOrderExecutor(param.Symbol)
	if err != nil {
		return err
	}

	// Calculate position size based on risk management
	quantity, err := s.calculatePositionSize(ctx, param)
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

	createdOrders, err := executor.SubmitOrders(ctx, order)
	if err != nil {
		log.WithError(err).Errorf("failed to submit market order: %+v", order)
		return err
	}
	log.Infof("created orders: %+v", createdOrders)
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
func (s *Strategy) calculatePositionSize(ctx context.Context, param OpenPositionParam) (fixedpoint.Value, error) {
	market, ok := s.Session.Market(param.Symbol)
	if !ok {
		return fixedpoint.Zero, fmt.Errorf("market %s not found", param.Symbol)
	}

	// Check if stop loss is provided, if not return original quantity
	if param.StopLossPrice.IsZero() {
		return param.Quantity, nil
	}

	// Get current market price
	ticker, err := s.Session.Exchange.QueryTicker(ctx, param.Symbol)
	if err != nil {
		return fixedpoint.Zero, fmt.Errorf("failed to get ticker for %s: %w", param.Symbol, err)
	}

	currentPrice := s.getCurrentPrice(ticker, param.Side)
	if currentPrice.IsZero() {
		return fixedpoint.Zero, fmt.Errorf("invalid current price for %s", param.Symbol)
	}

	riskPerUnit := s.calculateRiskPerUnit(currentPrice, param.StopLossPrice, param.Side)
	if riskPerUnit.Sign() <= 0 {
		return fixedpoint.Zero, s.createInvalidStopLossError(param.Side, currentPrice)
	}

	availableBalance, err := s.getAvailableBalance(market, param.Side)
	if err != nil {
		return fixedpoint.Zero, err
	}

	// Calculate maximum quantity based on MaxLossLimit
	var maxQuantityByRisk fixedpoint.Value
	if !s.MaxLossLimit.IsZero() {
		maxQuantityByRisk = s.MaxLossLimit.Div(riskPerUnit)
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

	log.Infof("Position size calculation: symbol=%s, currentPrice=%s, stopLoss=%s, riskPerUnit=%s, maxLossLimit=%s, availableBalance=%s, finalQuantity=%s",
		param.Symbol, currentPrice, param.StopLossPrice, riskPerUnit, s.MaxLossLimit, availableBalance, quantity)

	return quantity, nil
}

// getCurrentPrice returns the appropriate price based on order side
func (s *Strategy) getCurrentPrice(ticker *types.Ticker, side types.SideType) fixedpoint.Value {
	var price fixedpoint.Value
	if side == types.SideTypeBuy {
		price = ticker.Sell // Use ask price for buy orders
		if price.IsZero() {
			price = ticker.Last
		}
	} else {
		price = ticker.Buy // Use bid price for sell orders
		if price.IsZero() {
			price = ticker.Last
		}
	}
	return price
}

// calculateRiskPerUnit calculates the risk per unit based on current price, stop loss, and side
func (s *Strategy) calculateRiskPerUnit(currentPrice, stopLossPrice fixedpoint.Value, side types.SideType) fixedpoint.Value {
	if side == types.SideTypeBuy {
		// For long positions, risk is current price - stop loss price
		return currentPrice.Sub(stopLossPrice)
	}
	// For short positions, risk is stop loss price - current price
	return stopLossPrice.Sub(currentPrice)
}

// createInvalidStopLossError creates an appropriate error message for invalid stop loss prices
func (s *Strategy) createInvalidStopLossError(side types.SideType, currentPrice fixedpoint.Value) error {
	if side == types.SideTypeBuy {
		return fmt.Errorf("invalid stop loss price for buy order: stop loss should be below current price (%s)", currentPrice.String())
	}
	return fmt.Errorf("invalid stop loss price for sell order: stop loss should be above current price (%s)", currentPrice.String())
}

// getAvailableBalance returns the available balance for the appropriate currency based on order side
func (s *Strategy) getAvailableBalance(market types.Market, side types.SideType) (fixedpoint.Value, error) {
	account := s.Session.GetAccount()

	var currency string
	if side == types.SideTypeBuy {
		currency = market.QuoteCurrency // For buy orders, we need quote currency balance
	} else {
		currency = market.BaseCurrency // For sell orders, we need base currency balance
	}

	balance, ok := account.Balance(currency)
	if !ok {
		return fixedpoint.Zero, fmt.Errorf("no balance found for %s", currency)
	}

	return balance.Available, nil
}

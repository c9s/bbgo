package rebalance

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "rebalance"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

func instanceID(symbol string) string {
	return fmt.Sprintf("%s:%s", ID, symbol)
}

type Strategy struct {
	Environment *bbgo.Environment

	Interval      types.Interval   `json:"interval"`
	QuoteCurrency string           `json:"quoteCurrency"`
	TargetWeights types.ValueMap   `json:"targetWeights"`
	Threshold     fixedpoint.Value `json:"threshold"`
	MaxAmount     fixedpoint.Value `json:"maxAmount"` // max amount to buy or sell per order
	OrderType     types.OrderType  `json:"orderType"`
	DryRun        bool             `json:"dryRun"`
	OnStart       bool             `json:"onStart"` // rebalance on start

	PositionMap    PositionMap    `persistence:"positionMap"`
	ProfitStatsMap ProfitStatsMap `persistence:"profitStatsMap"`

	session          *bbgo.ExchangeSession
	orderExecutorMap GeneralOrderExecutorMap
	activeOrderBook  *bbgo.ActiveOrderBook
}

func (s *Strategy) Defaults() error {
	if s.OrderType == "" {
		s.OrderType = types.OrderTypeLimitMaker
	}
	return nil
}

func (s *Strategy) Initialize() error {
	return nil
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Validate() error {
	if len(s.TargetWeights) == 0 {
		return fmt.Errorf("targetWeights should not be empty")
	}

	if !s.TargetWeights.Sum().Eq(fixedpoint.One) {
		return fmt.Errorf("the sum of targetWeights should be 1")
	}

	for currency, weight := range s.TargetWeights {
		if weight.Float64() < 0 {
			return fmt.Errorf("%s weight: %f should not less than 0", currency, weight.Float64())
		}
	}

	if s.Threshold.Sign() < 0 {
		return fmt.Errorf("threshold should not less than 0")
	}

	if s.MaxAmount.Sign() < 0 {
		return fmt.Errorf("maxAmount shoud not less than 0")
	}
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	for _, symbol := range s.symbols() {
		session.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{Interval: s.Interval})
	}
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.session = session

	markets, err := s.markets()
	if err != nil {
		return err
	}

	if s.PositionMap == nil {
		s.PositionMap = make(PositionMap)
	}
	s.PositionMap.CreatePositions(markets)

	if s.ProfitStatsMap == nil {
		s.ProfitStatsMap = make(ProfitStatsMap)
	}
	s.ProfitStatsMap.CreateProfitStats(markets)

	s.orderExecutorMap = NewGeneralOrderExecutorMap(session, s.PositionMap)
	s.orderExecutorMap.BindEnvironment(s.Environment)
	s.orderExecutorMap.BindProfitStats(s.ProfitStatsMap)
	s.orderExecutorMap.Bind()
	s.orderExecutorMap.Sync(ctx, s)

	s.activeOrderBook = bbgo.NewActiveOrderBook("")
	s.activeOrderBook.BindStream(s.session.UserDataStream)

	session.UserDataStream.OnStart(func() {
		if s.OnStart {
			s.rebalance(ctx)
		}
	})

	s.session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		s.rebalance(ctx)
	})

	// the shutdown handler, you can cancel all orders
	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		_ = s.orderExecutorMap.GracefulCancel(ctx)
	})

	return nil
}

func (s *Strategy) rebalance(ctx context.Context) {
	// cancel active orders before rebalance
	if err := s.session.Exchange.CancelOrders(ctx, s.activeOrderBook.Orders()...); err != nil {
		log.WithError(err).Errorf("failed to cancel orders")
	}

	submitOrders, err := s.generateSubmitOrders(ctx)
	if err != nil {
		log.WithError(err).Error("failed to generate submit orders")
		return
	}
	for _, order := range submitOrders {
		log.Infof("generated submit order: %s", order.String())
	}

	if s.DryRun {
		log.Infof("dry run, not submitting orders")
		return
	}

	createdOrders, err := s.orderExecutorMap.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		log.WithError(err).Error("failed to submit orders")
		return
	}
	s.activeOrderBook.Add(createdOrders...)
}

func (s *Strategy) prices(ctx context.Context) (types.ValueMap, error) {
	m := make(types.ValueMap)
	for currency := range s.TargetWeights {
		if currency == s.QuoteCurrency {
			m[s.QuoteCurrency] = fixedpoint.One
			continue
		}

		ticker, err := s.session.Exchange.QueryTicker(ctx, currency+s.QuoteCurrency)
		if err != nil {
			return nil, err
		}

		m[currency] = ticker.Buy.Add(ticker.Sell).Div(fixedpoint.NewFromFloat(2.0))
	}
	return m, nil
}

func (s *Strategy) balances() (types.BalanceMap, error) {
	m := make(types.BalanceMap)
	balances := s.session.GetAccount().Balances()
	for currency := range s.TargetWeights {
		balance, ok := balances[currency]
		if !ok {
			return nil, fmt.Errorf("no balance for %s", currency)
		}
		m[currency] = balance
	}
	return m, nil
}

func (s *Strategy) generateSubmitOrders(ctx context.Context) (submitOrders []types.SubmitOrder, err error) {
	prices, err := s.prices(ctx)
	if err != nil {
		return nil, err
	}
	balances, err := s.balances()
	if err != nil {
		return nil, err
	}
	marketValues := prices.Mul(balanceToTotal(balances))
	currentWeights := marketValues.Normalize()

	for currency, targetWeight := range s.TargetWeights {
		if currency == s.QuoteCurrency {
			continue
		}

		symbol := currency + s.QuoteCurrency
		currentWeight := currentWeights[currency]
		currentPrice := prices[currency]

		log.Infof("%s price: %v, current weight: %v, target weight: %v",
			symbol,
			currentPrice,
			currentWeight,
			targetWeight)

		// calculate the difference between current weight and target weight
		// if the difference is less than threshold, then we will not create the order
		weightDifference := targetWeight.Sub(currentWeight)
		if weightDifference.Abs().Compare(s.Threshold) < 0 {
			log.Infof("%s weight distance |%v - %v| = |%v| less than the threshold: %v",
				symbol,
				currentWeight,
				targetWeight,
				weightDifference,
				s.Threshold)
			continue
		}

		quantity := weightDifference.Mul(marketValues.Sum()).Div(currentPrice)

		side := types.SideTypeBuy
		if quantity.Sign() < 0 {
			side = types.SideTypeSell
			quantity = quantity.Abs()
		}

		maxAmount := s.adjustMaxAmountByBalance(side, currency, currentPrice, balances)
		if maxAmount.Sign() > 0 {
			quantity = bbgo.AdjustQuantityByMaxAmount(quantity, currentPrice, maxAmount)
			log.Infof("adjust the quantity %v (%s %s @ %v) by max amount %v",
				quantity,
				symbol,
				side.String(),
				currentPrice,
				s.MaxAmount)
		}

		log.Debugf("symbol: %v, quantity: %v", symbol, quantity)

		order := types.SubmitOrder{
			Symbol:   symbol,
			Side:     side,
			Type:     s.OrderType,
			Quantity: quantity,
			Price:    currentPrice,
		}

		if ok := s.checkMinimalOrderQuantity(order); ok {
			submitOrders = append(submitOrders, order)
		}
	}

	return submitOrders, err
}

func (s *Strategy) symbols() (symbols []string) {
	for currency := range s.TargetWeights {
		if currency == s.QuoteCurrency {
			continue
		}
		symbols = append(symbols, currency+s.QuoteCurrency)
	}
	return symbols
}

func (s *Strategy) markets() ([]types.Market, error) {
	markets := []types.Market{}
	for _, symbol := range s.symbols() {
		market, ok := s.session.Market(symbol)
		if !ok {
			return nil, fmt.Errorf("market %s not found", symbol)
		}
		markets = append(markets, market)
	}
	return markets, nil
}

func (s *Strategy) adjustMaxAmountByBalance(side types.SideType, currency string, currentPrice fixedpoint.Value, balances types.BalanceMap) fixedpoint.Value {
	var maxAmount fixedpoint.Value

	switch side {
	case types.SideTypeBuy:
		maxAmount = balances[s.QuoteCurrency].Available
	case types.SideTypeSell:
		maxAmount = balances[currency].Available.Mul(currentPrice)
	default:
		log.Errorf("unknown side type: %s", side)
		return fixedpoint.Zero
	}

	if s.MaxAmount.Sign() > 0 {
		maxAmount = fixedpoint.Min(s.MaxAmount, maxAmount)
	}

	return maxAmount
}

func (s *Strategy) checkMinimalOrderQuantity(order types.SubmitOrder) bool {
	if order.Quantity.Compare(order.Market.MinQuantity) < 0 {
		log.Infof("order quantity is too small: %f < %f", order.Quantity.Float64(), order.Market.MinQuantity.Float64())
		return false
	}

	if order.Quantity.Mul(order.Price).Compare(order.Market.MinNotional) < 0 {
		log.Infof("order min notional is too small: %f < %f", order.Quantity.Mul(order.Price).Float64(), order.Market.MinNotional.Float64())
		return false
	}
	return true
}

func balanceToTotal(balances types.BalanceMap) types.ValueMap {
	m := make(types.ValueMap)
	for _, b := range balances {
		m[b.Currency] = b.Total()
	}
	return m
}

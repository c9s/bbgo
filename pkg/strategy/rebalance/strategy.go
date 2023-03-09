package rebalance

import (
	"context"
	"fmt"

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

type Strategy struct {
	Environment *bbgo.Environment

	Interval      types.Interval   `json:"interval"`
	QuoteCurrency string           `json:"quoteCurrency"`
	TargetWeights types.ValueMap   `json:"targetWeights"`
	Threshold     fixedpoint.Value `json:"threshold"`
	MaxAmount     fixedpoint.Value `json:"maxAmount"` // max amount to buy or sell per order
	OrderType     types.OrderType  `json:"orderType"`
	DryRun        bool             `json:"dryRun"`

	PositionMap    map[string]*types.Position    `persistence:"positionMap"`
	ProfitStatsMap map[string]*types.ProfitStats `persistence:"profitStatsMap"`

	session          *bbgo.ExchangeSession
	orderExecutorMap map[string]*bbgo.GeneralOrderExecutor
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

func (s *Strategy) InstanceID(symbol string) string {
	return fmt.Sprintf("%s:%s", ID, symbol)
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
		s.initPositionMapFromMarkets(markets)
	}

	if s.ProfitStatsMap == nil {
		s.initProfitStatsMapFromMarkets(markets)
	}

	s.initOrderExecutorMapFromMarkets(ctx, markets)

	s.activeOrderBook = bbgo.NewActiveOrderBook("")
	s.activeOrderBook.BindStream(s.session.UserDataStream)

	s.session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		s.rebalance(ctx)
	})

	return nil
}

func (s *Strategy) initPositionMapFromMarkets(markets []types.Market) {
	s.PositionMap = make(map[string]*types.Position)
	for _, market := range markets {
		position := types.NewPositionFromMarket(market)
		position.Strategy = s.ID()
		position.StrategyInstanceID = s.InstanceID(market.Symbol)
		s.PositionMap[market.Symbol] = position
	}
}

func (s *Strategy) initProfitStatsMapFromMarkets(markets []types.Market) {
	s.ProfitStatsMap = make(map[string]*types.ProfitStats)
	for _, market := range markets {
		s.ProfitStatsMap[market.Symbol] = types.NewProfitStats(market)
	}
}

func (s *Strategy) initOrderExecutorMapFromMarkets(ctx context.Context, markets []types.Market) {
	s.orderExecutorMap = make(map[string]*bbgo.GeneralOrderExecutor)
	for _, market := range markets {
		symbol := market.Symbol

		orderExecutor := bbgo.NewGeneralOrderExecutor(s.session, symbol, ID, s.InstanceID(symbol), s.PositionMap[symbol])
		orderExecutor.BindEnvironment(s.Environment)
		orderExecutor.BindProfitStats(s.ProfitStatsMap[symbol])
		orderExecutor.Bind()
		orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
			bbgo.Sync(ctx, s)
		})

		s.orderExecutorMap[market.Symbol] = orderExecutor
	}
}

func (s *Strategy) rebalance(ctx context.Context) {
	// cancel active orders before rebalance
	if err := s.session.Exchange.CancelOrders(ctx, s.activeOrderBook.Orders()...); err != nil {
		log.WithError(err).Errorf("failed to cancel orders")
	}

	submitOrders := s.generateSubmitOrders(ctx)
	for _, order := range submitOrders {
		log.Infof("generated submit order: %s", order.String())
	}

	if s.DryRun {
		return
	}

	for _, submitOrder := range submitOrders {
		createdOrders, err := s.orderExecutorMap[submitOrder.Symbol].SubmitOrders(ctx, submitOrder)
		if err != nil {
			log.WithError(err).Error("failed to submit orders")
			return
		}
		s.activeOrderBook.Add(createdOrders...)
	}
}

func (s *Strategy) prices(ctx context.Context) types.ValueMap {
	m := make(types.ValueMap)
	for currency := range s.TargetWeights {
		if currency == s.QuoteCurrency {
			m[s.QuoteCurrency] = fixedpoint.One
			continue
		}

		ticker, err := s.session.Exchange.QueryTicker(ctx, currency+s.QuoteCurrency)
		if err != nil {
			log.WithError(err).Error("failed to query tickers")
			return nil
		}

		m[currency] = ticker.Last
	}
	return m
}

func (s *Strategy) quantities() types.ValueMap {
	m := make(types.ValueMap)

	balances := s.session.GetAccount().Balances()
	for currency := range s.TargetWeights {
		m[currency] = balances[currency].Total()
	}

	return m
}

func (s *Strategy) generateSubmitOrders(ctx context.Context) (submitOrders []types.SubmitOrder) {
	prices := s.prices(ctx)
	marketValues := prices.Mul(s.quantities())
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

		if s.MaxAmount.Sign() > 0 {
			quantity = bbgo.AdjustQuantityByMaxAmount(quantity, currentPrice, s.MaxAmount)
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

		submitOrders = append(submitOrders, order)
	}

	return submitOrders
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

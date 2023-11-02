package rebalance

import (
	"context"
	"fmt"
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "rebalance"

var log = logrus.WithField("strategy", ID)
var two = fixedpoint.NewFromFloat(2.0)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

func instanceID(symbol string) string {
	return fmt.Sprintf("%s:%s", ID, symbol)
}

type Strategy struct {
	*MultiMarketStrategy

	Environment *bbgo.Environment

	CronExpression string           `json:"cronExpression"`
	QuoteCurrency  string           `json:"quoteCurrency"`
	TargetWeights  types.ValueMap   `json:"targetWeights"`
	Threshold      fixedpoint.Value `json:"threshold"`
	MaxAmount      fixedpoint.Value `json:"maxAmount"` // max amount to buy or sell per order
	OrderType      types.OrderType  `json:"orderType"`
	DryRun         bool             `json:"dryRun"`
	OnStart        bool             `json:"onStart"` // rebalance on start

	session         *bbgo.ExchangeSession
	symbols         []string
	markets         map[string]types.Market
	activeOrderBook *bbgo.ActiveOrderBook
	cron            *cron.Cron
}

func (s *Strategy) Defaults() error {
	if s.OrderType == "" {
		s.OrderType = types.OrderTypeLimitMaker
	}
	return nil
}

func (s *Strategy) Initialize() error {
	for currency := range s.TargetWeights {
		if currency == s.QuoteCurrency {
			continue
		}

		s.symbols = append(s.symbols, currency+s.QuoteCurrency)
	}
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

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.session = session

	s.markets = make(map[string]types.Market)
	for _, symbol := range s.symbols {
		market, ok := s.session.Market(symbol)
		if !ok {
			return fmt.Errorf("market %s not found", symbol)
		}
		s.markets[symbol] = market
	}

	s.MultiMarketStrategy = &MultiMarketStrategy{}
	s.MultiMarketStrategy.Initialize(ctx, s.Environment, session, s.markets, ID)

	s.activeOrderBook = bbgo.NewActiveOrderBook("")
	s.activeOrderBook.BindStream(s.session.UserDataStream)

	session.UserDataStream.OnStart(func() {
		if s.OnStart {
			s.rebalance(ctx)
		}
	})

	// the shutdown handler, you can cancel all orders
	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		_ = s.OrderExecutorMap.GracefulCancel(ctx)
	})

	s.cron = cron.New()
	s.cron.AddFunc(s.CronExpression, func() {
		s.rebalance(ctx)
	})
	s.cron.Start()

	return nil
}

func (s *Strategy) rebalance(ctx context.Context) {
	// cancel active orders before rebalance
	if err := s.session.Exchange.CancelOrders(ctx, s.activeOrderBook.Orders()...); err != nil {
		log.WithError(err).Errorf("failed to cancel orders")
	}

	order, err := s.generateOrder(ctx)
	if err != nil {
		log.WithError(err).Error("failed to generate order")
		return
	}

	if order == nil {
		log.Info("no order generated")
		return
	}
	log.Infof("generated order: %s", order.String())

	if s.DryRun {
		log.Infof("dry run, not submitting orders")
		return
	}

	createdOrders, err := s.OrderExecutorMap.SubmitOrders(ctx, *order)
	if err != nil {
		log.WithError(err).Error("failed to submit orders")
		return
	}
	s.activeOrderBook.Add(createdOrders...)
}

func (s *Strategy) queryMidPrices(ctx context.Context) (types.ValueMap, error) {
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

		m[currency] = ticker.Buy.Add(ticker.Sell).Div(two)
	}
	return m, nil
}

func (s *Strategy) selectBalances() (types.BalanceMap, error) {
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

func (s *Strategy) generateOrder(ctx context.Context) (*types.SubmitOrder, error) {
	prices, err := s.queryMidPrices(ctx)
	if err != nil {
		return nil, err
	}

	balances, err := s.selectBalances()
	if err != nil {
		return nil, err
	}

	values := prices.Mul(toValueMap(balances))
	weights := values.Normalize()

	for symbol, market := range s.markets {
		target := s.TargetWeights[market.BaseCurrency]
		weight := weights[market.BaseCurrency]
		midPrice := prices[market.BaseCurrency]

		log.Infof("%s mid price: %s", symbol, midPrice.String())
		log.Infof("%s weight: %.2f%%, target: %.2f%%", market.BaseCurrency, weight.Float64()*100, target.Float64()*100)

		// calculate the difference between current weight and target weight
		// if the difference is less than threshold, then we will not create the order
		diff := target.Sub(weight)
		if diff.Abs().Compare(s.Threshold) < 0 {
			log.Infof("%s weight is close to target, skip", market.BaseCurrency)
			continue
		}

		quantity := diff.Mul(values.Sum()).Div(midPrice)

		side := types.SideTypeBuy
		if quantity.Sign() < 0 {
			side = types.SideTypeSell
			quantity = quantity.Abs()
		}

		if s.MaxAmount.Float64() > 0 {
			quantity = bbgo.AdjustQuantityByMaxAmount(quantity, midPrice, s.MaxAmount)
			log.Infof("adjust quantity %s (%s %s @ %s) by max amount %s",
				quantity.String(),
				symbol,
				side.String(),
				midPrice.String(),
				s.MaxAmount.String())
		}

		if side == types.SideTypeBuy {
			quantity = fixedpoint.Min(quantity, balances[s.QuoteCurrency].Available.Div(midPrice))
		} else if side == types.SideTypeSell {
			quantity = fixedpoint.Min(quantity, balances[market.BaseCurrency].Available)
		}

		if market.IsDustQuantity(quantity, midPrice) {
			log.Infof("quantity %s (%s %s @ %s) is dust quantity, skip",
				quantity.String(),
				symbol,
				side.String(),
				midPrice.String())
			continue
		}

		return &types.SubmitOrder{
			Symbol:   symbol,
			Side:     side,
			Type:     s.OrderType,
			Quantity: quantity,
			Price:    midPrice,
		}, nil
	}
	return nil, nil
}

func toValueMap(balances types.BalanceMap) types.ValueMap {
	m := make(types.ValueMap)
	for _, b := range balances {
		// m[b.Currency] = b.Net()
		m[b.Currency] = b.Available
	}
	return m
}

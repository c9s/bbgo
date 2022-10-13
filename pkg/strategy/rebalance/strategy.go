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
	Interval      types.Interval   `json:"interval"`
	QuoteCurrency string           `json:"quoteCurrency"`
	TargetWeights types.ValueMap   `json:"targetWeights"`
	Threshold     fixedpoint.Value `json:"threshold"`
	DryRun        bool             `json:"dryRun"`
	// max amount to buy or sell per order
	MaxAmount fixedpoint.Value `json:"maxAmount"`

	activeOrderBook *bbgo.ActiveOrderBook
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
	session.Subscribe(types.KLineChannel, s.symbols()[0], types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.activeOrderBook = bbgo.NewActiveOrderBook("")
	s.activeOrderBook.BindStream(session.UserDataStream)

	markets := session.Markets()
	for _, symbol := range s.symbols() {
		if _, ok := markets[symbol]; !ok {
			return fmt.Errorf("exchange: %s does not supoort matket: %s", session.Exchange.Name(), symbol)
		}
	}

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		s.rebalance(ctx, orderExecutor, session)
	})

	return nil
}

func (s *Strategy) rebalance(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	// cancel active orders before rebalance
	if err := session.Exchange.CancelOrders(ctx, s.activeOrderBook.Orders()...); err != nil {
		log.WithError(err).Errorf("failed to cancel orders")
	}

	submitOrders := s.generateSubmitOrders(ctx, session)
	for _, order := range submitOrders {
		log.Infof("generated submit order: %s", order.String())
	}

	if s.DryRun {
		return
	}

	createdOrders, err := orderExecutor.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		log.WithError(err).Error("failed to submit orders")
		return
	}

	s.activeOrderBook.Add(createdOrders...)
}

func (s *Strategy) prices(ctx context.Context, session *bbgo.ExchangeSession) types.ValueMap {
	m := make(types.ValueMap)

	tickers, err := session.Exchange.QueryTickers(ctx, s.symbols()...)
	if err != nil {
		log.WithError(err).Error("failed to query tickers")
		return nil
	}

	for currency := range s.TargetWeights {
		if currency == s.QuoteCurrency {
			m[s.QuoteCurrency] = fixedpoint.One
			continue
		}
		m[currency] = tickers[currency+s.QuoteCurrency].Last
	}

	return m
}

func (s *Strategy) quantities(session *bbgo.ExchangeSession) types.ValueMap {
	m := make(types.ValueMap)

	balances := session.GetAccount().Balances()
	for currency := range s.TargetWeights {
		m[currency] = balances[currency].Total()
	}

	return m
}

func (s *Strategy) generateSubmitOrders(ctx context.Context, session *bbgo.ExchangeSession) (submitOrders []types.SubmitOrder) {
	prices := s.prices(ctx, session)
	marketValues := prices.Mul(s.quantities(session))
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
			Type:     types.OrderTypeLimit,
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

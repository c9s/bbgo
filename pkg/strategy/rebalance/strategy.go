package rebalance

import (
	"context"
	"fmt"
	"math"
	"sort"

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
	Notifiability *bbgo.Notifiability

	Interval      types.Interval              `json:"interval"`
	BaseCurrency  string                      `json:"baseCurrency"`
	TargetWeights map[string]fixedpoint.Value `json:"targetWeights"`
	Threshold     fixedpoint.Value            `json:"threshold"`
	Verbose       bool                        `json:"verbose"`
	DryRun        bool                        `json:"dryRun"`
	// max amount to buy or sell per order
	MaxAmount fixedpoint.Value `json:"maxAmount"`

	// sorted currencies
	currencies []string

	// symbol for subscribing kline
	symbol string

	activeOrderBook *bbgo.ActiveOrderBook
}

func (s *Strategy) Initialize() error {
	for currency := range s.TargetWeights {
		s.currencies = append(s.currencies, currency)
	}

	sort.Strings(s.currencies)

	s.symbol = s.currencies[0] + s.BaseCurrency

	return nil
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Validate() error {
	if len(s.TargetWeights) == 0 {
		return fmt.Errorf("targetWeights should not be empty")
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
	session.Subscribe(types.KLineChannel, s.symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.activeOrderBook = bbgo.NewActiveOrderBook("")
	s.activeOrderBook.BindStream(session.UserDataStream)

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// cancel active orders before rebalance
		if err := session.Exchange.CancelOrders(ctx, s.activeOrderBook.Orders()...); err != nil {
			log.WithError(err).Errorf("failed to cancel orders")
		}

		s.rebalance(ctx, orderExecutor, session)
	})
	return nil
}

func (s *Strategy) rebalance(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	prices, err := s.prices(ctx, session)
	if err != nil {
		return
	}

	marketValues := prices.Mul(s.quantities(session))

	submitOrders := s.generateSubmitOrders(prices, marketValues)
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

func (s *Strategy) prices(ctx context.Context, session *bbgo.ExchangeSession) (prices types.Float64Slice, err error) {
	tickers, err := session.Exchange.QueryTickers(ctx, s.symbols()...)
	if err != nil {
		return nil, err
	}

	for _, currency := range s.currencies {
		if currency == s.BaseCurrency {
			prices = append(prices, 1.0)
			continue
		}

		symbol := currency + s.BaseCurrency
		prices = append(prices, tickers[symbol].Last.Float64())
	}
	return prices, nil
}

func (s *Strategy) quantities(session *bbgo.ExchangeSession) (quantities types.Float64Slice) {
	balances := session.GetAccount().Balances()
	for _, currency := range s.currencies {
		quantities = append(quantities, balances[currency].Total().Float64())
	}
	return quantities
}

func (s *Strategy) generateSubmitOrders(prices, marketValues types.Float64Slice) (submitOrders []types.SubmitOrder) {
	currentWeights := marketValues.Normalize()

	for i, currency := range s.currencies {
		if currency == s.BaseCurrency {
			continue
		}

		symbol := currency + s.BaseCurrency
		currentWeight := currentWeights[i]
		currentPrice := prices[i]
		targetWeight := s.TargetWeights[currency].Float64()

		log.Infof("%s price: %v, current weight: %v, target weight: %v",
			symbol,
			currentPrice,
			currentWeight,
			targetWeight)

		// calculate the difference between current weight and target weight
		// if the difference is less than threshold, then we will not create the order
		weightDifference := targetWeight - currentWeight
		if math.Abs(weightDifference) < s.Threshold.Float64() {
			log.Infof("%s weight distance |%v - %v| = |%v| less than the threshold: %v",
				symbol,
				currentWeight,
				targetWeight,
				weightDifference,
				s.Threshold)
			continue
		}

		quantity := fixedpoint.NewFromFloat((weightDifference * marketValues.Sum()) / currentPrice)

		side := types.SideTypeBuy
		if quantity.Sign() < 0 {
			side = types.SideTypeSell
			quantity = quantity.Abs()
		}

		if s.MaxAmount.Sign() > 0 {
			quantity = bbgo.AdjustQuantityByMaxAmount(quantity, fixedpoint.NewFromFloat(currentPrice), s.MaxAmount)
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
			Price:    fixedpoint.NewFromFloat(currentPrice),
		}

		submitOrders = append(submitOrders, order)
	}
	return submitOrders
}

func (s *Strategy) symbols() (symbols []string) {
	for _, currency := range s.currencies {
		if currency == s.BaseCurrency {
			continue
		}
		symbols = append(symbols, currency+s.BaseCurrency)
	}
	return symbols
}

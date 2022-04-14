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
	IgnoreLocked  bool                        `json:"ignoreLocked"`
	Verbose       bool                        `json:"verbose"`
	DryRun        bool                        `json:"dryRun"`
	// max amount to buy or sell per order
	MaxAmount fixedpoint.Value `json:"maxAmount"`

	currencies []string
}

func (s *Strategy) Initialize() error {
	for currency := range s.TargetWeights {
		s.currencies = append(s.currencies, currency)
	}

	sort.Strings(s.currencies)
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
	for _, symbol := range s.getSymbols() {
		session.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{Interval: s.Interval.String()})
	}
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		s.rebalance(ctx, orderExecutor, session)
	})
	return nil
}

func (s *Strategy) rebalance(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	prices, err := s.getPrices(ctx, session)
	if err != nil {
		return
	}

	balances := session.Account.Balances()
	quantities := s.getQuantities(balances)
	marketValues := prices.Mul(quantities)

	orders := s.generateSubmitOrders(prices, marketValues)
	for _, order := range orders {
		log.Infof("generated submit order: %s", order.String())
	}

	if s.DryRun {
		return
	}

	_, err = orderExecutor.SubmitOrders(ctx, orders...)
	if err != nil {
		log.WithError(err).Error("submit order error")
		return
	}
}

func (s *Strategy) getPrices(ctx context.Context, session *bbgo.ExchangeSession) (prices types.Float64Slice, err error) {
	for _, currency := range s.currencies {
		if currency == s.BaseCurrency {
			prices = append(prices, 1.0)
			continue
		}

		symbol := currency + s.BaseCurrency
		ticker, err := session.Exchange.QueryTicker(ctx, symbol)
		if err != nil {
			s.Notifiability.Notify("query ticker error: %s", err.Error())
			log.WithError(err).Error("query ticker error")
			return prices, err
		}

		prices = append(prices, ticker.Last.Float64())
	}
	return prices, nil
}

func (s *Strategy) getQuantities(balances types.BalanceMap) (quantities types.Float64Slice) {
	for _, currency := range s.currencies {
		if s.IgnoreLocked {
			quantities = append(quantities, balances[currency].Total().Float64())
		} else {
			quantities = append(quantities, balances[currency].Available.Float64())
		}
	}
	return quantities
}

func (s *Strategy) generateSubmitOrders(prices, marketValues types.Float64Slice) (submitOrders []types.SubmitOrder) {
	currentWeights := marketValues.Normalize()
	totalValue := marketValues.Sum()

	log.Infof("total value: %f", totalValue)

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

		quantity := fixedpoint.NewFromFloat((weightDifference * totalValue) / currentPrice)

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
			Type:     types.OrderTypeMarket,
			Quantity: quantity}

		submitOrders = append(submitOrders, order)
	}
	return submitOrders
}

func (s *Strategy) getSymbols() (symbols []string) {
	for _, currency := range s.currencies {
		if currency == s.BaseCurrency {
			continue
		}
		symbols = append(symbols, currency+s.BaseCurrency)
	}
	return symbols
}

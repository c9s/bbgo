package kline

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

const ID = "rebalance"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

func Sum(m map[string]fixedpoint.Value) fixedpoint.Value {
	sum := fixedpoint.NewFromFloat(0.0)
	for _, v := range m {
		sum = sum.Add(v)
	}
	return sum
}

func Normalize(m map[string]fixedpoint.Value) map[string]fixedpoint.Value {
	sum := Sum(m)
	if sum.Float64() == 1.0 {
		return m
	}

	normalized := make(map[string]fixedpoint.Value)
	for k, v := range m {
		normalized[k] = v.Div(sum)
	}
	return normalized
}

func ElementwiseProduct(m1, m2 map[string]fixedpoint.Value) map[string]fixedpoint.Value {
	m := make(map[string]fixedpoint.Value)
	for k, v := range m1 {
		m[k] = v.Mul(m2[k])
	}
	return m
}

type Strategy struct {
	Notifiability *bbgo.Notifiability

	Interval      types.Duration              `json:"interval"`
	BaseCurrency  string                      `json:"baseCurrency"`
	TargetWeights map[string]fixedpoint.Value `json:"targetWeights"`
	Threshold     fixedpoint.Value            `json:"threshold"`
	IgnoreLocked  bool                        `json:"ignoreLocked"`
	Verbose       bool                        `json:"verbose"`

	// max amount to buy or sell per order
	MaxAmount fixedpoint.Value `json:"maxAmount"`
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Validate() error {
	if s.Interval == 0 {
		return fmt.Errorf("interval shoud not be 0")
	}

	if len(s.TargetWeights) == 0 {
		return fmt.Errorf("targetWeights should not be empty")
	}

	for currency, weight := range s.TargetWeights {
		if weight.Float64() < 0 {
			return fmt.Errorf("%s weight: %f should not less than 0", currency, weight.Float64())
		}
	}

	if s.Threshold < 0 {
		return fmt.Errorf("threshold should not less than 0")
	}

	if s.MaxAmount < 0 {
		return fmt.Errorf("maxAmount shoud not less than 0")
	}

	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.TargetWeights = Normalize(s.TargetWeights)

	go func() {
		ticker := time.NewTicker(util.MillisecondsJitter(s.Interval.Duration(), 1000))
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				s.rebalance(ctx, orderExecutor, session)
			}
		}
	}()

	return nil
}

func (s *Strategy) rebalance(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	prices, err := s.getPrices(ctx, session)
	if err != nil {
		return
	}

	balances := session.Account.Balances()
	quantities := s.getQuantities(balances)
	marketValues := ElementwiseProduct(prices, quantities)

	orders := s.generateSubmitOrders(prices, marketValues)
	_, err = orderExecutor.SubmitOrders(ctx, orders...)
	if err != nil {
		log.WithError(err).Error("submit order error")
		return
	}
}

func (s *Strategy) getPrices(ctx context.Context, session *bbgo.ExchangeSession) (map[string]fixedpoint.Value, error) {
	prices := make(map[string]fixedpoint.Value)

	for currency := range s.TargetWeights {
		if currency == s.BaseCurrency {
			prices[currency] = fixedpoint.NewFromFloat(1.0)
			continue
		}

		symbol := currency + s.BaseCurrency
		ticker, err := session.Exchange.QueryTicker(ctx, symbol)
		if err != nil {
			s.Notifiability.Notify("query ticker error: %s", err.Error())
			log.WithError(err).Error("query ticker error")
			return prices, err
		}

		prices[currency] = fixedpoint.NewFromFloat(ticker.Last)
	}
	return prices, nil
}

func (s *Strategy) getQuantities(balances types.BalanceMap) map[string]fixedpoint.Value {
	quantities := make(map[string]fixedpoint.Value)
	for currency := range s.TargetWeights {
		if s.IgnoreLocked {
			quantities[currency] = balances[currency].Total()
		} else {
			quantities[currency] = balances[currency].Available
		}
	}
	return quantities
}

func (s *Strategy) generateSubmitOrders(prices, marketValues map[string]fixedpoint.Value) []types.SubmitOrder {
	var submitOrders []types.SubmitOrder

	currentWeights := Normalize(marketValues)
	totalValue := Sum(marketValues)

	log.Infof("total value: %f", totalValue.Float64())

	for currency, targetWeight := range s.TargetWeights {
		if currency == s.BaseCurrency {
			continue
		}
		symbol := currency + s.BaseCurrency
		currentWeight := currentWeights[currency]
		currentPrice := prices[currency]
		log.Infof("%s price: %f, current weight: %f, target weight: %f",
			symbol,
			currentPrice.Float64(),
			currentWeight.Float64(),
			targetWeight.Float64())

		// calculate the difference between current weight and target weight
		// if the difference is less than threshold, then we will not create the order
		weightDifference := targetWeight.Sub(currentWeight)
		if weightDifference.Abs() < s.Threshold {
			log.Infof("%s weight distance |%f - %f| = |%f| less than the threshold: %f",
				currentWeight.Float64(),
				targetWeight.Float64(),
				weightDifference.Float64(),
				s.Threshold.Float64())
			continue
		}

		quantity := weightDifference.Mul(totalValue).Div(currentPrice)

		side := types.SideTypeBuy
		if quantity < 0.0 {
			side = types.SideTypeSell
			quantity = quantity.Abs()
		}

		if s.MaxAmount > 0 {
			quantity = bbgo.AdjustQuantityByMaxAmount(quantity, currentPrice, s.MaxAmount)
			log.Infof("adjust the quantity %f (%s %s @ %f) by max amount %f",
				quantity.Float64(),
				symbol,
				side.String(),
				currentPrice.Float64(),
				s.MaxAmount.Float64())
		}

		order := types.SubmitOrder{
			Symbol:   symbol,
			Side:     side,
			Type:     types.OrderTypeMarket,
			Quantity: quantity.Float64()}

		submitOrders = append(submitOrders, order)
	}
	return submitOrders
}

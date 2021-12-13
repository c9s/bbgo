package kline

import (
	"context"
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

	Interval     types.Duration              `json:"interval"`
	BaseCurrency string                      `json:"baseCurrency"`
	Weights      map[string]fixedpoint.Value `json:"weights"`
	Threshold    fixedpoint.Value            `json:"threshold"`
	IgnoreLocked bool                        `json:"ignoreLocked"`
	Verbose      bool                        `json:"verbose"`
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.Weights = Normalize(s.Weights)

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
		return
	}
}

func (s *Strategy) getPrices(ctx context.Context, session *bbgo.ExchangeSession) (map[string]fixedpoint.Value, error) {
	prices := make(map[string]fixedpoint.Value)

	for currency := range s.Weights {
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
	for currency := range s.Weights {
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

	for currency, target := range s.Weights {
		if currency == s.BaseCurrency {
			continue
		}
		symbol := currency + s.BaseCurrency
		weight := currentWeights[currency]
		price := prices[currency]

		diff := target.Sub(weight)
		if diff.Abs() < s.Threshold {
			continue
		}

		quantity := diff.Mul(totalValue).Div(price)

		side := types.SideTypeBuy
		if quantity < 0.0 {
			side = types.SideTypeSell
			quantity = quantity.Abs()
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

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

type Asset struct {
	Currency string
	Quantity fixedpoint.Value
	Price    fixedpoint.Value
}

func (a *Asset) MarketValue() fixedpoint.Value {
	return a.Quantity.Mul(a.Price)
}

type Portfolio struct {
	Assets       map[string]Asset
	BaseCurrency string
}

func (p *Portfolio) TotalValue() fixedpoint.Value {
	v := fixedpoint.NewFromFloat(0.0)
	for _, a := range p.Assets {
		v = v.Add(a.MarketValue())
	}
	return v
}

func (p *Portfolio) Weights() map[string]fixedpoint.Value {
	weights := make(map[string]fixedpoint.Value)
	value := p.TotalValue()
	for currency, asset := range p.Assets {
		weights[currency] = asset.MarketValue().Div(value)
	}
	return weights
}

func (p *Portfolio) PrintValue() {
	value := p.TotalValue()
	log.Infof("portfolio value: %f %s", value.Float64(), p.BaseCurrency)
}

func (p *Portfolio) PrintWeights() {
	weights := p.Weights()
	for currency, weight := range weights {
		weightInPercent := weight.Float64() * 100.0
		log.Infof("%s: %.2f%%", currency, weightInPercent)
	}
}

type Strategy struct {
	Notifiability *bbgo.Notifiability

	Interval        types.Duration              `json:"interval"`
	BaseCurrency    string                      `json:"baseCurrency"`
	TargetPortfolio map[string]fixedpoint.Value `json:"targetPortfolio"`
	Threshold       fixedpoint.Value            `json:"threshold"`
	IgnoreLocked    bool                        `json:"ignoreLocked"`
	Verbose         bool                        `json:"verbose"`

	Portfolio Portfolio
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if s.Verbose {
		s.Notifiability.Notify("Start to rebalance the portfolio")
	}

	s.NormalizeTargetWeights()

	go func() {
		ticker := time.NewTicker(util.MillisecondsJitter(s.Interval.Duration(), 1000))
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				s.Rebalance(ctx, orderExecutor, session)
			}
		}
	}()

	return nil
}

func (s *Strategy) NormalizeTargetWeights() {
	sum := fixedpoint.NewFromFloat(0.0)
	for _, w := range s.TargetPortfolio {
		sum = sum.Add(w)
	}

	if sum.Float64() != 1.0 {
		log.Infof("sum of weights: %f != 1.0", sum.Float64())
	}

	for currency, w := range s.TargetPortfolio {
		s.TargetPortfolio[currency] = w.Div(sum)
	}

}

func (s *Strategy) Rebalance(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	prices, err := s.GetPrices(ctx, session)
	if err != nil {
		return
	}

	balances := session.Account.Balances()
	s.UpdateCurrentPortfolio(balances, prices)

	s.Portfolio.PrintValue()
	s.Portfolio.PrintWeights()

	orders := s.generateSubmitOrders(prices)
	_, err = orderExecutor.SubmitOrders(ctx, orders...)
	if err != nil {
		return
	}
}

func (s *Strategy) GetPrices(ctx context.Context, session *bbgo.ExchangeSession) (map[string]fixedpoint.Value, error) {
	prices := make(map[string]fixedpoint.Value)

	for currency := range s.TargetPortfolio {
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

func (s *Strategy) UpdateCurrentPortfolio(balances types.BalanceMap, prices map[string]fixedpoint.Value) {
	assets := make(map[string]Asset)
	for currency := range s.TargetPortfolio {

		qty := balances[currency].Available
		if s.IgnoreLocked {
			qty = balances[currency].Total()
		}

		assets[currency] = Asset{currency, qty, prices[currency]}
	}
	s.Portfolio = Portfolio{assets, s.BaseCurrency}
}

func (s *Strategy) generateSubmitOrders(prices map[string]fixedpoint.Value) []types.SubmitOrder {
	var submitOrders []types.SubmitOrder

	currentWeights := s.Portfolio.Weights()
	for currency, target := range s.TargetPortfolio {
		if currency == s.BaseCurrency {
			continue
		}
		symbol := currency + s.BaseCurrency
		weight := currentWeights[currency]
		price := prices[currency]

		diff := target - weight
		if diff.Abs() < s.Threshold {
			continue
		}

		quantity := diff.Mul(s.Portfolio.TotalValue()).Div(price)

		side := types.SideTypeBuy
		if quantity < 0 {
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

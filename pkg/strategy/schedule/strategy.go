package schedule

import (
	"context"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "schedule"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Market types.Market

	Notifiability *bbgo.Notifiability

	// StandardIndicatorSet contains the standard indicators of a market (symbol)
	// This field will be injected automatically since we defined the Symbol field.
	*bbgo.StandardIndicatorSet

	// Interval is the period that you want to submit order
	Interval types.Interval `json:"interval"`

	// Symbol is the symbol of the market
	Symbol string `json:"symbol"`

	// Side is the order side type, which can be buy or sell
	Side types.SideType `json:"side,omitempty"`

	bbgo.QuantityOrAmount

	BelowMovingAverage *bbgo.MovingAverageSettings `json:"belowMovingAverage,omitempty"`

	AboveMovingAverage *bbgo.MovingAverageSettings `json:"aboveMovingAverage,omitempty"`
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval.String()})
	if s.BelowMovingAverage != nil {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.BelowMovingAverage.Interval.String()})
	}
	if s.AboveMovingAverage != nil {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.AboveMovingAverage.Interval.String()})
	}
}

func (s *Strategy) Validate() error {
	if err := s.QuantityOrAmount.Validate(); err != nil {
		return err
	}

	return nil
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if s.StandardIndicatorSet == nil {
		return errors.New("StandardIndicatorSet can not be nil, injection failed?")
	}

	var belowMA types.Float64Indicator
	var aboveMA types.Float64Indicator
	var err error
	if s.BelowMovingAverage != nil {
		belowMA, err = s.BelowMovingAverage.Indicator(s.StandardIndicatorSet)
		if err != nil {
			return err
		}
	}

	if s.AboveMovingAverage != nil {
		aboveMA, err = s.AboveMovingAverage.Indicator(s.StandardIndicatorSet)
		if err != nil {
			return err
		}
	}

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != s.Symbol {
			return
		}

		if kline.Interval != s.Interval {
			return
		}

		closePrice := fixedpoint.NewFromFloat(kline.Close)
		quantity := s.QuantityOrAmount.CalculateQuantity(closePrice)
		side := s.Side

		if s.BelowMovingAverage != nil || s.AboveMovingAverage != nil {

			match := false
			// if any of the conditions satisfies then we execute order
			if belowMA != nil && closePrice.Float64() < belowMA.Last() {
				match = true
				if s.BelowMovingAverage != nil {
					if s.BelowMovingAverage.Side != nil {
						side = *s.BelowMovingAverage.Side
					}

					// override the default quantity or amount
					if s.BelowMovingAverage.QuantityOrAmount.IsSet() {
						quantity = s.BelowMovingAverage.QuantityOrAmount.CalculateQuantity(closePrice)
					}
				}
			} else if aboveMA != nil && closePrice.Float64() > aboveMA.Last() {
				match = true
				if s.AboveMovingAverage != nil {
					if s.AboveMovingAverage.Side != nil {
						side = *s.AboveMovingAverage.Side
					}

					if s.AboveMovingAverage.QuantityOrAmount.IsSet() {
						quantity = s.AboveMovingAverage.QuantityOrAmount.CalculateQuantity(closePrice)
					}
				}
			}

			if !match {
				s.Notifiability.Notify("skip, the %s closed price %f is below or above moving average", s.Symbol, closePrice.Float64())
				return
			}
		}

		// calculate quote quantity for balance checking
		quoteQuantity := quantity.Mul(closePrice)

		// execute orders
		switch side {
		case types.SideTypeBuy:
			quoteBalance, ok := session.Account.Balance(s.Market.QuoteCurrency)
			if !ok {
				log.Errorf("can not place scheduled %s order, quote balance %s is empty", s.Symbol, s.Market.QuoteCurrency)
				return
			}

			if quoteBalance.Available < quoteQuantity {
				s.Notifiability.Notify("Can not place scheduled %s order: quote balance %s is not enough: %f < %f", s.Symbol, s.Market.QuoteCurrency, quoteBalance.Available.Float64(), quoteQuantity.Float64())
				log.Errorf("can not place scheduled %s order: quote balance %s is not enough: %f < %f", s.Symbol, s.Market.QuoteCurrency, quoteBalance.Available.Float64(), quoteQuantity.Float64())
				return
			}

		case types.SideTypeSell:
			baseBalance, ok := session.Account.Balance(s.Market.BaseCurrency)
			if !ok {
				log.Errorf("can not place scheduled %s order, base balance %s is empty", s.Symbol, s.Market.BaseCurrency)
				return
			}

			if baseBalance.Available < quantity {
				s.Notifiability.Notify("Can not place scheduled %s order: base balance %s is not enough: %f < %f", s.Symbol, s.Market.QuoteCurrency, baseBalance.Available.Float64(), quantity.Float64())
				log.Errorf("can not place scheduled %s order: base balance %s is not enough: %f < %f", s.Symbol, s.Market.QuoteCurrency, baseBalance.Available.Float64(), quantity.Float64())
				return
			}

		}

		s.Notifiability.Notify("Submitting scheduled %s order with quantity %f at price %f", s.Symbol, quantity.Float64(), closePrice.Float64())
		_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     side,
			Type:     types.OrderTypeMarket,
			Quantity: quantity.Float64(),
			Market:   s.Market,
		})
		if err != nil {
			s.Notifiability.Notify("Can not place scheduled %s order: submit error %s", s.Symbol, err.Error())
			log.WithError(err).Errorf("can not place scheduled %s order error", s.Symbol)
		}
	})

	return nil
}

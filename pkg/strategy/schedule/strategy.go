package schedule

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "schedule"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Market types.Market

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

	MaxBaseBalance fixedpoint.Value `json:"maxBaseBalance"`

	BelowMovingAverage *bbgo.MovingAverageSettings `json:"belowMovingAverage,omitempty"`

	AboveMovingAverage *bbgo.MovingAverageSettings `json:"aboveMovingAverage,omitempty"`

	Position *types.Position `persistence:"position"`

	session       *bbgo.ExchangeSession
	orderExecutor *bbgo.GeneralOrderExecutor
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	if s.BelowMovingAverage != nil {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.BelowMovingAverage.Interval})
	}
	if s.AboveMovingAverage != nil {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.AboveMovingAverage.Interval})
	}
}

func (s *Strategy) Validate() error {
	if err := s.QuantityOrAmount.Validate(); err != nil {
		return err
	}

	return nil
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.session = session

	if s.StandardIndicatorSet == nil {
		return errors.New("StandardIndicatorSet can not be nil, injection failed?")
	}

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	instanceID := s.InstanceID()
	s.orderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})
	s.orderExecutor.Bind()

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

		closePrice := kline.Close
		closePriceF := closePrice.Float64()
		quantity := s.QuantityOrAmount.CalculateQuantity(closePrice)
		side := s.Side

		if s.BelowMovingAverage != nil || s.AboveMovingAverage != nil {

			match := false
			// if any of the conditions satisfies then we execute order
			if belowMA != nil && closePriceF < belowMA.Last() {
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
			} else if aboveMA != nil && closePriceF > aboveMA.Last() {
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
				bbgo.Notify("skip, the %s closed price %v is below or above moving average", s.Symbol, closePrice)
				return
			}
		}

		// calculate quote quantity for balance checking
		quoteQuantity := quantity.Mul(closePrice)

		// execute orders
		switch side {
		case types.SideTypeBuy:
			if !s.MaxBaseBalance.IsZero() {
				if baseBalance, ok := session.GetAccount().Balance(s.Market.BaseCurrency); ok {
					total := baseBalance.Total()
					if total.Add(quantity).Compare(s.MaxBaseBalance) >= 0 {
						quantity = s.MaxBaseBalance.Sub(total)
						quoteQuantity = quantity.Mul(closePrice)
					}
				}
			}

			quoteBalance, ok := session.GetAccount().Balance(s.Market.QuoteCurrency)
			if !ok {
				log.Errorf("can not place scheduled %s order, quote balance %s is empty", s.Symbol, s.Market.QuoteCurrency)
				return
			}

			if quoteBalance.Available.Compare(quoteQuantity) < 0 {
				bbgo.Notify("Can not place scheduled %s order: quote balance %s is not enough: %v < %v", s.Symbol, s.Market.QuoteCurrency, quoteBalance.Available, quoteQuantity)
				log.Errorf("can not place scheduled %s order: quote balance %s is not enough: %v < %v", s.Symbol, s.Market.QuoteCurrency, quoteBalance.Available, quoteQuantity)
				return
			}

		case types.SideTypeSell:
			baseBalance, ok := session.GetAccount().Balance(s.Market.BaseCurrency)
			if !ok {
				log.Errorf("can not place scheduled %s order, base balance %s is empty", s.Symbol, s.Market.BaseCurrency)
				return
			}

			quantity = fixedpoint.Min(quantity, baseBalance.Available)
			quoteQuantity = quantity.Mul(closePrice)
		}

		if s.Market.IsDustQuantity(quantity, closePrice) {
			log.Warnf("%s: quantity %f is too small", s.Symbol, quantity.Float64())
			return
		}

		bbgo.Notify("Submitting scheduled %s order with quantity %s at price %s", s.Symbol, quantity.String(), closePrice.String())
		_, err := s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     side,
			Type:     types.OrderTypeMarket,
			Quantity: quantity,
			Market:   s.Market,
		})
		if err != nil {
			bbgo.Notify("Can not place scheduled %s order: submit error %s", s.Symbol, err.Error())
			log.WithError(err).Errorf("can not place scheduled %s order error", s.Symbol)
		}
	})

	return nil
}

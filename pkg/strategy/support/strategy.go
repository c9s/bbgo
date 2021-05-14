package support

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// support -- support and targets
const ID = "support"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Target struct {
	ProfitPercentage      float64                         `json:"profitPercentage"`
	QuantityPercentage    float64                         `json:"quantityPercentage"`
	MarginOrderSideEffect types.MarginOrderSideEffectType `json:"marginOrderSideEffect"`
}

type Strategy struct {
	*bbgo.Notifiability

	Symbol                string                          `json:"symbol"`
	Interval              types.Interval                  `json:"interval"`
	MovingAverageWindow   int                             `json:"movingAverageWindow"`
	Quantity              fixedpoint.Value                `json:"quantity"`
	MinVolume             fixedpoint.Value                `json:"minVolume"`
	MarginOrderSideEffect types.MarginOrderSideEffectType `json:"marginOrderSideEffect"`
	Targets               []Target                        `json:"targets"`

	ScaleQuantity *bbgo.PriceVolumeScale `json:"scaleQuantity"`
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Validate() error {
	if s.Quantity == 0 && s.ScaleQuantity == nil {
		return fmt.Errorf("quantity or scaleQuantity can not be zero")
	}

	if s.MinVolume == 0 {
		return fmt.Errorf("minVolume can not be zero")
	}

	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: string(s.Interval)})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// set default values
	if s.Interval == "" {
		s.Interval = types.Interval5m
	}

	if s.MovingAverageWindow == 0 {
		s.MovingAverageWindow = 99
	}

	// buy when price drops -8%
	market, ok := session.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("market %s is not defined", s.Symbol)
	}

	standardIndicatorSet, ok := session.StandardIndicatorSet(s.Symbol)
	if !ok {
		return fmt.Errorf("standardIndicatorSet is nil, symbol %s", s.Symbol)
	}

	var iw = types.IntervalWindow{Interval: s.Interval, Window: s.MovingAverageWindow}
	var ema = standardIndicatorSet.EWMA(iw)

	session.Stream.OnKLineClosed(func(kline types.KLine) {
		// skip k-lines from other symbols
		if kline.Symbol != s.Symbol {
			return
		}

		closePrice := kline.GetClose()
		if closePrice > ema.Last() {
			return
		}

		if kline.Volume < s.MinVolume.Float64() {
			return
		}

		s.Notify("found support: close price %f is under EMA %f, volume %f > minimum volume %f", closePrice, ema.Last(), kline.Volume, s.MinVolume.Float64())

		var quantity float64
		if s.Quantity > 0 {
			quantity = s.Quantity.Float64()
		} else if s.ScaleQuantity != nil {
			var err error
			quantity, err = s.ScaleQuantity.Scale(closePrice, kline.Volume)
			if err != nil {
				log.WithError(err).Error(err.Error())
				return
			}
		}

		// for spot, we need to modify the quantity
		if !session.Margin {
			minNotional := closePrice * 1.003 * quantity
			b, ok := session.Account.Balance(market.QuoteCurrency)
			if !ok {
				log.Errorf("balance %s not found", market.QuoteCurrency)
				return
			}

			if minNotional > b.Available.Float64() {
				log.Warnf("modifying quantity %f according to the min quote balance %f %s", quantity, b.Available.Float64(), market.QuoteCurrency)
				quantity = bbgo.AdjustFloatQuantityByMaxAmount(quantity, closePrice, b.Available.Float64())
			}
		}

		orderForm := types.SubmitOrder{
			Symbol:           s.Symbol,
			Market:           market,
			Side:             types.SideTypeBuy,
			Type:             types.OrderTypeMarket,
			Quantity:         quantity,
			MarginSideEffect: s.MarginOrderSideEffect,
		}

		_, err := orderExecutor.SubmitOrders(ctx, orderForm)
		if err != nil {
			log.WithError(err).Error("submit order error")
			return
		}

		// submit target orders
		var targetOrders []types.SubmitOrder
		for _, target := range s.Targets {
			targetPrice := closePrice * (1.0 + target.ProfitPercentage)
			targetQuantity := quantity * target.QuantityPercentage
			targetOrders = append(targetOrders, types.SubmitOrder{
				Symbol:   kline.Symbol,
				Market:   market,
				Type:     types.OrderTypeLimit,
				Side:     types.SideTypeSell,
				Price:    targetPrice,
				Quantity: targetQuantity,

				MarginSideEffect: target.MarginOrderSideEffect,
				TimeInForce:      "GTC",
			})
		}

		_, err = orderExecutor.SubmitOrders(ctx, targetOrders...)
		if err != nil {
			log.WithError(err).Error("submit profit target order error")
		}
	})

	return nil
}

package sat

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// sat -- support and targets
const ID = "supportAndTargets"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Target struct {
	ProfitPercentage   float64 `json:"profitPercentage"`
	QuantityPercentage float64 `json:"quantityPercentage"`
}

type Strategy struct {
	Symbol              string           `json:"symbol"`
	Interval            types.Interval   `json:"interval"`
	MovingAverageWindow int              `json:"movingAverageWindow"`
	Quantity            fixedpoint.Value `json:"quantity"`
	MinVolume           fixedpoint.Value `json:"minVolume"`
	Targets             []Target         `json:"targets"`
}

func (s *Strategy) ID() string {
	return ID
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

	if s.Quantity == 0 {
		return fmt.Errorf("quantity can not be zero")
	}

	if s.MinVolume == 0 {
		return fmt.Errorf("minVolume can not be zero")
	}

	// buy when price drops -8%
	market, ok := session.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("market %s is not defined", s.Symbol)
	}

	_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Symbol:           s.Symbol,
		Market:           market,
		Type:             types.OrderTypeLimit,
		Side:             types.SideTypeSell,
		Price:            27.0,
		Quantity:         0.5,
		MarginSideEffect: types.SideEffectTypeAutoRepay,
		TimeInForce:      "GTC",
	})
	if err != nil {
		log.WithError(err).Error("submit profit target order error")
		return err
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

		log.Infof("found support: close price %f is under EMA %f, volume %f > minimum volume %f", closePrice, ema.Last(), kline.Volume, s.MinVolume.Float64())

		quantity := s.Quantity.Float64()
		_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
			Symbol:   s.Symbol,
			Market:   market,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeMarket,
			Quantity: quantity,

			// This is for Long position.
			MarginSideEffect: types.SideEffectTypeMarginBuy,
		})
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

				// This is for Long position.
				MarginSideEffect: types.SideEffectTypeAutoRepay,
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

package pricedrop

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "pricedrop"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Symbol string `json:"symbol"`

	Interval          types.Interval   `json:"interval"`
	BaseQuantity      fixedpoint.Value `json:"baseQuantity"`
	MinDropPercentage fixedpoint.Value `json:"minDropPercentage"`
	MinDropChange     fixedpoint.Value `json:"minDropChange"`

	MovingAverageWindow int `json:"movingAverageWindow"`
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
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

	standardIndicatorSet := session.StandardIndicatorSet(s.Symbol)
	if !ok {
		return fmt.Errorf("standardIndicatorSet is nil, symbol %s", s.Symbol)
	}

	var iw = types.IntervalWindow{Interval: s.Interval, Window: s.MovingAverageWindow}
	var ema = standardIndicatorSet.EWMA(iw)

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// skip k-lines from other symbols
		if kline.Symbol != s.Symbol {
			return
		}

		change := kline.GetChange()

		// skip positive change
		if change.Sign() > 0 {
			return
		}

		if kline.Close.Float64() > ema.Last() {
			log.Warnf("kline close price %v is above EMA %s %f", kline.Close, ema.IntervalWindow, ema.Last())
			return
		}

		changeP := change.Div(kline.Open).Abs()

		if !s.MinDropPercentage.IsZero() {
			if changeP.Compare(s.MinDropPercentage.Abs()) < 0 {
				return
			}
		} else if !s.MinDropChange.IsZero() {
			if change.Abs().Compare(s.MinDropChange.Abs()) < 0 {
				return
			}
		} else {
			// not configured, we shall skip
			log.Warnf("parameters are not configured, skipping action...")
			return
		}

		quantity := s.BaseQuantity.Mul(fixedpoint.One.Add(changeP))
		_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
			Symbol:   kline.Symbol,
			Market:   market,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeMarket,
			Quantity: quantity,
		})
		if err != nil {
			log.WithError(err).Error("submit order error")
		}
	})

	return nil
}

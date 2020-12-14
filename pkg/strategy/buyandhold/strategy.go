package buyandhold

import (
	"context"
	"fmt"
	"math"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var log = logrus.WithField("strategy", "buyandhold")

func init() {
	bbgo.RegisterStrategy("buyandhold", &Strategy{})
}

type Strategy struct {
	Symbol string `json:"symbol"`

	Interval          string           `json:"interval"`
	BaseQuantity      float64          `json:"baseQuantity"`
	MinDropPercentage fixedpoint.Value `json:"minDropPercentage"`
	MinDropChange     fixedpoint.Value `json:"minDropChange"`
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// buy when price drops -8%
	market, ok := session.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("market %s is not defined", s.Symbol)
	}

	session.Stream.OnKLine(func(kline types.KLine) {
		// skip k-lines from other symbols
		if kline.Symbol != s.Symbol {
			return
		}

		changePercentage := kline.GetChange() / kline.Open
		log.Infof("change %f <=> %f", changePercentage, s.MinDropPercentage.Float64())
	})

	session.Stream.OnKLineClosed(func(kline types.KLine) {
		// skip k-lines from other symbols
		if kline.Symbol != s.Symbol {
			return
		}

		change := kline.GetChange()

		// skip positive change
		if change > 0 {
			return
		}

		changeP := change / kline.Open

		if s.MinDropPercentage != 0 {
			if math.Abs(changeP) < math.Abs(s.MinDropPercentage.Float64()) {
				return
			}
		} else if s.MinDropChange != 0 {
			if math.Abs(change) < math.Abs(s.MinDropChange.Float64()) {
				return
			}
		} else {
			// not configured, we shall skip
			log.Warnf("parameters are not configured, skipping action...")
			return
		}

		quantity := s.BaseQuantity * (1.0 + math.Abs(changeP))
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

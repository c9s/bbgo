package buyandhold

import (
	"context"
	"math"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	bbgo.RegisterExchangeStrategy("buyandhold", &Strategy{})
}

type Strategy struct {
	Symbol            string  `json:"symbol"`
	Interval          string  `json:"interval"`
	BaseQuantity      float64 `json:"baseQuantity"`
	MinDropPercentage float64 `json:"minDropPercentage"`
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})

	session.Stream.OnKLine(func(kline types.KLine) {
		// skip k-lines from other symbols
		if kline.Symbol != s.Symbol {
			return
		}

		changePercentage := kline.GetChange() / kline.Open
		log.Infof("change %f <=> %f", changePercentage, s.MinDropPercentage)
	})

	session.Stream.OnKLineClosed(func(kline types.KLine) {
		// skip k-lines from other symbols
		if kline.Symbol != s.Symbol {
			return
		}

		changePercentage := kline.GetChange() / kline.Open

		if changePercentage > s.MinDropPercentage {
			return
		}

		// buy when price drops -8%
		market, ok := session.Market(s.Symbol)
		if !ok {
			log.Warnf("market %s is not defined", s.Symbol)
			return
		}

		quantity := s.BaseQuantity * (1.0 + math.Abs(changePercentage))
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

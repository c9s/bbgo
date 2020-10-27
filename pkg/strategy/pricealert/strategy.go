package pricealert

import (
	"context"
	"math"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	bbgo.RegisterExchangeStrategy("pricealert", &Strategy{})
}

type Strategy struct {
	bbgo.Notifiability

	Symbol    string `json:"symbol"`
	Interval  string `json:"interval"`
	MinChange float64 `json:"minChange"`
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	session.Stream.OnKLine(func(kline types.KLine) {
		market, ok := session.Market(kline.Symbol)
		if !ok {
			return
		}

		if math.Abs(kline.GetChange()) > s.MinChange {
			if channel, ok := s.RouteSymbol(s.Symbol); ok {
				_ = s.NotifyTo(channel, "%s hit price %s, change %f", s.Symbol, market.FormatPrice(kline.Close), kline.GetChange())
			} else {
				_ = s.Notify("%s hit price %s, change %f", s.Symbol, market.FormatPrice(kline.Close), kline.GetChange())
			}
		}
	})
	return nil
}

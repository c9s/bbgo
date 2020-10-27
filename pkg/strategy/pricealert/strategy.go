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
	// The notification system will be injected into the strategy automatically.
	bbgo.Notifiability

	// These fields will be filled from the config file (it translates YAML to JSON)
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
				s.NotifyTo(channel, "%s hit price %s, change %f", s.Symbol, market.FormatPrice(kline.Close), kline.GetChange())
			} else {
				s.Notify("%s hit price %s, change %f", s.Symbol, market.FormatPrice(kline.Close), kline.GetChange())
			}
		}
	})
	return nil
}

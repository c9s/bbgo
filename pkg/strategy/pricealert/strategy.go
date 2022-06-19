package pricealert

import (
	"context"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "pricealert"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	// These fields will be filled from the config file (it translates YAML to JSON)
	Symbol    string           `json:"symbol"`
	Interval  types.Interval   `json:"interval"`
	MinChange fixedpoint.Value `json:"minChange"`
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	session.MarketDataStream.OnKLine(func(kline types.KLine) {
		market, ok := session.Market(kline.Symbol)
		if !ok {
			return
		}

		if kline.GetChange().Abs().Compare(s.MinChange) > 0 {
			if channel, ok := bbgo.Notification.RouteSymbol(s.Symbol); ok {
				bbgo.NotifyTo(channel, "%s hit price %s, change %v", s.Symbol, market.FormatPrice(kline.Close), kline.GetChange())
			} else {
				bbgo.Notify("%s hit price %s, change %v", s.Symbol, market.FormatPrice(kline.Close), kline.GetChange())
			}
		}
	})
	return nil
}

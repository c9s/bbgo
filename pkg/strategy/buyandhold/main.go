package buyandhold

import (
	"context"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

type Strategy struct {
	symbol string
}

func New(symbol string) *Strategy {
	return &Strategy{
		symbol: symbol,
	}
}

func (s *Strategy) Run(ctx context.Context, trader types.Trader, session *bbgo.ExchangeSession) error {
	session.Subscribe(types.KLineChannel, s.symbol, types.SubscribeOptions{ Interval: "1h" })

	session.Stream.OnKLineClosed(func(kline types.KLine) {
		changePercentage := kline.GetChange() / kline.Open

		// buy when price drops -10%
		if changePercentage < -0.1 {
			trader.SubmitOrder(ctx, &types.SubmitOrder{
				Symbol:         kline.Symbol,
				Side:           types.SideTypeBuy,
				Type:           types.OrderTypeMarket,
				Quantity:       1.0,
			})
		}
	})

	return nil
}






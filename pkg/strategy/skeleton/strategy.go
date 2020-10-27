package skeleton

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	bbgo.RegisterExchangeStrategy("skeleton", &Strategy{})
}

type Strategy struct {
	Symbol string `json:"symbol"`
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
	session.Stream.OnKLineClosed(func(kline types.KLine) {
		market, ok := session.Market(s.Symbol)
		if !ok {
			return
		}

		quoteBalance, ok := session.Account.Balance(market.QuoteCurrency)
		if !ok {
			return
		}
		_ = quoteBalance

		_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
			Symbol:   kline.Symbol,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeMarket,
			Quantity: 0.01,
		})

		if err != nil {
			log.WithError(err).Error("submit order error")
		}
	})

	return nil
}

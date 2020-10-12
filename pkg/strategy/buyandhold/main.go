package buyandhold

import (
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

func (s *Strategy) Run(trader types.Trader, session *bbgo.ExchangeSession) error {
	session.Subscribe(types.KLineChannel, s.symbol, types.SubscribeOptions{})
	session.Stream.OnKLineClosed(func(kline types.KLine) {
		// trader.SubmitOrder(ctx, ....)
	})

	return nil
}






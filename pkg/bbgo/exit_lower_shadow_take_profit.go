package bbgo

import (
	"context"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type LowerShadowTakeProfit struct {
	// inherit from the strategy
	types.IntervalWindow

	// inherit from the strategy
	Symbol string `json:"symbol"`

	Ratio         fixedpoint.Value `json:"ratio"`
	session       *ExchangeSession
	orderExecutor *GeneralOrderExecutor
}

func (s *LowerShadowTakeProfit) Subscribe(session *ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *LowerShadowTakeProfit) Bind(session *ExchangeSession, orderExecutor *GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	position := orderExecutor.Position()
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != position.Symbol || kline.Interval != types.Interval1m {
			return
		}

		closePrice := kline.Close
		if position.IsClosed() || position.IsDust(closePrice) {
			return
		}

		roi := position.ROI(closePrice)
		if roi.Sign() < 0 {
			return
		}

		if s.Ratio.IsZero() {
			return
		}

		if kline.GetLowerShadowHeight().Div(kline.Close).Compare(s.Ratio) > 0 {
			Notify("%s TakeProfit triggered by shadow ratio %f, price = %f",
				position.Symbol,
				kline.GetLowerShadowRatio().Float64(),
				kline.Close.Float64(),
				kline)

			_ = orderExecutor.ClosePosition(context.Background(), fixedpoint.One)
			return
		}
	})
}

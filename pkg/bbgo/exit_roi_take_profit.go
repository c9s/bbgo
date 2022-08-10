package bbgo

import (
	"context"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// RoiTakeProfit force takes the profit by the given ROI percentage.
type RoiTakeProfit struct {
	Symbol     string           `json:"symbol"`
	Percentage fixedpoint.Value `json:"percentage"`

	session       *ExchangeSession
	orderExecutor *GeneralOrderExecutor
}

func (s *RoiTakeProfit) Subscribe(session *ExchangeSession) {
	// use 1m kline to handle roi stop
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})
}

func (s *RoiTakeProfit) Bind(session *ExchangeSession, orderExecutor *GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	position := orderExecutor.Position()
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval1m, func(kline types.KLine) {
		closePrice := kline.Close
		if position.IsClosed() || position.IsDust(closePrice) {
			return
		}

		roi := position.ROI(closePrice)
		if roi.Compare(s.Percentage) >= 0 {
			// stop loss
			Notify("[RoiTakeProfit] %s take profit is triggered by ROI %s/%s, price: %f", position.Symbol, roi.Percentage(), s.Percentage.Percentage(), kline.Close.Float64())
			_ = orderExecutor.ClosePosition(context.Background(), fixedpoint.One, "roiTakeProfit")
			return
		}
	}))
}

package bbgo

import (
	"context"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// RoiTakeProfit force takes the profit by the given ROI percentage.
type RoiTakeProfit struct {
	Symbol             string           `json:"symbol"`
	Percentage         fixedpoint.Value `json:"percentage"`
	CancelActiveOrders bool             `json:"cancelActiveOrders"`

	// Interval is the time resolution to update the stop order
	// KLine per Interval will be used for updating the stop order
	Interval types.Interval `json:"interval,omitempty"`

	session       *ExchangeSession
	orderExecutor *GeneralOrderExecutor
}

func (s *RoiTakeProfit) Subscribe(session *ExchangeSession) {
	// use kline to handle roi stop
	if s.Interval == "" {
		s.Interval = types.Interval1m
	}
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *RoiTakeProfit) Bind(session *ExchangeSession, orderExecutor *GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	position := orderExecutor.Position()
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {
		closePrice := kline.Close
		if position.IsClosed() || position.IsDust(closePrice) || position.IsClosing() {
			return
		}

		roi := position.ROI(closePrice)
		if roi.Compare(s.Percentage) >= 0 {
			// stop loss
			Notify("[RoiTakeProfit] %s take profit is triggered by ROI %s/%s, price: %f", position.Symbol, roi.Percentage(), s.Percentage.Percentage(), kline.Close.Float64())
			if s.CancelActiveOrders {
				_ = s.orderExecutor.GracefulCancel(context.Background())
			}
			_ = orderExecutor.ClosePosition(context.Background(), fixedpoint.One, "roiTakeProfit")
			return
		}
	}))
}

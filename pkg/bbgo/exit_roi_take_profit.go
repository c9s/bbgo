package bbgo

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// RoiTakeProfit force takes the profit by the given ROI percentage.
type RoiTakeProfit struct {
	Symbol     string           `json:"symbol"`
	Percentage fixedpoint.Value `json:"percentage"`

	// Partial is a percentage of the position to be closed
	// 50% means you will close halt the position
	Partial fixedpoint.Value `json:"partial,omitempty"`

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
		if roi.Compare(s.Percentage) < 0 {
			return
		}

		// default to close the whole position
		percent := one
		if !s.Partial.IsZero() {
			percent = s.Partial
		}

		// stop loss
		Notify("[RoiTakeProfit] %s take profit is triggered by ROI %s/%s, price: %f, closing: %s",
			position.Symbol,
			roi.Percentage(),
			s.Percentage.Percentage(),
			kline.Close.Float64(),
			percent.Percentage())

		if err := orderExecutor.ClosePosition(context.Background(), percent, "roiTakeProfit"); err != nil {
			log.WithError(err).Error("failed to close position")
		}
	}))
}

package bbgo

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type RoiStopLoss struct {
	Symbol string

	// Percentage is the ROI loss percentage
	// 1% means when you loss 1% and it will close the position
	Percentage fixedpoint.Value `json:"percentage"`

	// Partial is a percentage of the position to be closed
	// 50% means you will close halt the position
	Partial fixedpoint.Value `json:"partial,omitempty"`

	session       *ExchangeSession
	orderExecutor *GeneralOrderExecutor
}

func (s *RoiStopLoss) Subscribe(session *ExchangeSession) {
	// use 1m kline to handle roi stop
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})
}

func (s *RoiStopLoss) Bind(session *ExchangeSession, orderExecutor *GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	position := orderExecutor.Position()
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval1m, func(kline types.KLine) {
		s.checkStopPrice(kline.Close, position)
	}))

	if !IsBackTesting && enableMarketTradeStop {
		session.MarketDataStream.OnMarketTrade(func(trade types.Trade) {
			if trade.Symbol != position.Symbol {
				return
			}

			s.checkStopPrice(trade.Price, position)
		})
	}
}

func (s *RoiStopLoss) checkStopPrice(closePrice fixedpoint.Value, position *types.Position) {
	if position.IsClosed() || position.IsDust(closePrice) {
		return
	}

	roi := position.ROI(closePrice)
	// logrus.Debugf("ROIStopLoss: price=%f roi=%s stop=%s", closePrice.Float64(), roi.Percentage(), s.Percentage.Neg().Percentage())

	if roi.Compare(s.Percentage.Neg()) > 0 {
		return
	}

	// default to close the whole position
	percent := one
	if !s.Partial.IsZero() {
		percent = s.Partial
	}

	Notify("[RoiStopLoss] %s stop loss triggered by ROI %s/%s, price: %f, closing: %s",
		position.Symbol,
		roi.Percentage(),
		s.Percentage.Neg().Percentage(),
		closePrice.Float64(),
		percent.Percentage())

	if err := s.orderExecutor.ClosePosition(context.Background(), percent, "roiStopLoss"); err != nil {
		log.WithError(err).Error("failed to close position")
	}
}

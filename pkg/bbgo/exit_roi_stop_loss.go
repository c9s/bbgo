package bbgo

import (
	"context"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type RoiStopLoss struct {
	Symbol             string
	Percentage         fixedpoint.Value `json:"percentage"`
	CancelActiveOrders bool             `json:"cancelActiveOrders"`
	// Interval is the time resolution to update the stop order
	// KLine per Interval will be used for updating the stop order
	Interval types.Interval `json:"interval,omitempty"`

	session       *ExchangeSession
	orderExecutor *GeneralOrderExecutor
}

func (s *RoiStopLoss) Subscribe(session *ExchangeSession) {
	// use kline to handle roi stop
	if s.Interval == "" {
		s.Interval = types.Interval1m
	}
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *RoiStopLoss) Bind(session *ExchangeSession, orderExecutor *GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	position := orderExecutor.Position()
	f := func(kline types.KLine) {
		s.checkStopPrice(kline.Close, position)
	}

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, f))
	session.MarketDataStream.OnKLine(types.KLineWith(s.Symbol, s.Interval, f))

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
	if position.IsClosed() || position.IsDust(closePrice) || position.IsClosing() {
		return
	}

	roi := position.ROI(closePrice)
	// logrus.Debugf("ROIStopLoss: price=%f roi=%s stop=%s", closePrice.Float64(), roi.Percentage(), s.Percentage.Neg().Percentage())
	if roi.Compare(s.Percentage.Neg()) < 0 {
		// stop loss
		Notify("[RoiStopLoss] %s stop loss triggered by ROI %s/%s, currentPrice = %f", position.Symbol, roi.Percentage(), s.Percentage.Neg().Percentage(), closePrice.Float64())
		if s.CancelActiveOrders {
			_ = s.orderExecutor.GracefulCancel(context.Background())
		}
		_ = s.orderExecutor.ClosePosition(context.Background(), fixedpoint.One, "roiStopLoss")
		return
	}
}

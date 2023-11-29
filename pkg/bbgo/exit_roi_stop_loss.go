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
	f := func(kline types.KLine) {
		s.checkStopPrice(kline.Close, position)
	}

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval1m, f))
	session.MarketDataStream.OnKLine(types.KLineWith(s.Symbol, types.Interval1m, f))

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

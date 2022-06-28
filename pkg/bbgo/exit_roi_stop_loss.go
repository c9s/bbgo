package bbgo

import (
	"context"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type RoiStopLoss struct {
	Percentage fixedpoint.Value `json:"percentage"`

	session       *ExchangeSession
	orderExecutor *GeneralOrderExecutor
}

func (s *RoiStopLoss) Bind(session *ExchangeSession, orderExecutor *GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	position := orderExecutor.Position()
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != position.Symbol || kline.Interval != types.Interval1m {
			return
		}

		s.checkStopPrice(kline.Close, position)
	})

	if !IsBackTesting {
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
	if roi.Compare(s.Percentage.Neg()) < 0 {
		// stop loss
		Notify("[RoiStopLoss] %s stop loss triggered by ROI %s/%s, price: %f", position.Symbol, roi.Percentage(), s.Percentage.Neg().Percentage(), closePrice.Float64())
		_ = s.orderExecutor.ClosePosition(context.Background(), fixedpoint.One, "roiStopLoss")
		return
	}
}

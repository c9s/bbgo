package bbgo

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type TrailingStop2 struct {
	Symbol string

	// CallbackRate is the callback rate from the previous high price
	CallbackRate fixedpoint.Value `json:"callbackRate,omitempty"`

	// ClosePosition is a percentage of the position to be closed
	ClosePosition fixedpoint.Value `json:"closePosition,omitempty"`

	// MinProfit is the percentage of the minimum profit ratio.
	// Stop order will be activated only when the price reaches above this threshold.
	MinProfit fixedpoint.Value `json:"minProfit,omitempty"`

	// Interval is the time resolution to update the stop order
	// KLine per Interval will be used for updating the stop order
	Interval types.Interval `json:"interval,omitempty"`

	// private fields
	session       *ExchangeSession
	orderExecutor *GeneralOrderExecutor
}

func (s *TrailingStop2) Subscribe(session *ExchangeSession) {
	// use 1m kline to handle roi stop
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})
}

func (s *TrailingStop2) Bind(session *ExchangeSession, orderExecutor *GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	position := orderExecutor.Position()
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval1m, func(kline types.KLine) {
		s.checkStopPrice(kline.Close, position)
	}))

	if !IsBackTesting {
		session.MarketDataStream.OnMarketTrade(func(trade types.Trade) {
			if trade.Symbol != position.Symbol {
				return
			}

			s.checkStopPrice(trade.Price, position)
		})
	}
}

func (s *TrailingStop2) checkStopPrice(closePrice fixedpoint.Value, position *types.Position) {
	if position.IsClosed() || position.IsDust(closePrice) {
		return
	}

	/*
	roi := position.ROI(closePrice)
	if roi.Compare(s.CallbackRate.Neg()) < 0 {
		// stop loss
		Notify("[TrailingStop2] %s stop loss triggered by ROI %s/%s, price: %f", position.Symbol, roi.Percentage(), s.Percentage.Neg().Percentage(), closePrice.Float64())
		_ = s.orderExecutor.ClosePosition(context.Background(), fixedpoint.One, "TrailingStop2")
		return
	}
	*/
}

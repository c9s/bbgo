package bbgo

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type TrailingStop2 struct {
	Symbol string

	// CallbackRate is the callback rate from the previous high price
	CallbackRate fixedpoint.Value `json:"callbackRate,omitempty"`

	ActivationRatio fixedpoint.Value `json:"activationRatio,omitempty"`

	// ClosePosition is a percentage of the position to be closed
	ClosePosition fixedpoint.Value `json:"closePosition,omitempty"`

	// MinProfit is the percentage of the minimum profit ratio.
	// Stop order will be activated only when the price reaches above this threshold.
	MinProfit fixedpoint.Value `json:"minProfit,omitempty"`

	// Interval is the time resolution to update the stop order
	// KLine per Interval will be used for updating the stop order
	Interval types.Interval `json:"interval,omitempty"`

	Side types.SideType `json:"side,omitempty"`

	latestHigh fixedpoint.Value

	// activated: when the price reaches the min profit price, we set the activated to true to enable trailing stop
	activated bool

	// private fields
	session       *ExchangeSession
	orderExecutor *GeneralOrderExecutor
}

func (s *TrailingStop2) Subscribe(session *ExchangeSession) {
	// use 1m kline to handle roi stop
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *TrailingStop2) Bind(session *ExchangeSession, orderExecutor *GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor
	s.latestHigh = fixedpoint.Zero

	position := orderExecutor.Position()
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {
		if err := s.checkStopPrice(kline.Close, position); err != nil {
			log.WithError(err).Errorf("error")
		}
	}))

	if !IsBackTesting {
		session.MarketDataStream.OnMarketTrade(func(trade types.Trade) {
			if trade.Symbol != position.Symbol {
				return
			}

			if err := s.checkStopPrice(trade.Price, position); err != nil {
				log.WithError(err).Errorf("error")
			}
		})
	}
}

func (s *TrailingStop2) getRatio(price fixedpoint.Value, position *types.Position) (fixedpoint.Value, error) {
	switch s.Side {
	case types.SideTypeBuy:
		// for short position
		return position.AverageCost.Sub(price).Div(price), nil
	case types.SideTypeSell:
		return price.Sub(position.AverageCost).Div(position.AverageCost), nil
	}

	return fixedpoint.Zero, fmt.Errorf("unexpected side type: %v", s.Side)
}

func (s *TrailingStop2) checkStopPrice(price fixedpoint.Value, position *types.Position) error {
	if position.IsClosed() || position.IsDust(price) {
		return nil
	}

	if !s.MinProfit.IsZero() {
		// check if we have the minimal profit
		roi := position.ROI(price)
		if roi.Compare(s.MinProfit) >= 0 {
			Notify("[trailingStop] activated: ROI %f > minimal profit ratio %f", roi.Float64(), s.MinProfit.Float64())
			s.activated = true
		}
	} else if !s.ActivationRatio.IsZero() {
		ratio, err := s.getRatio(price, position)
		if err != nil {
			return err
		}

		if ratio.Compare(s.ActivationRatio) >= 0 {
			s.activated = true
		}
	}

	// update the latest high for the sell order, or the latest low for the buy order
	if s.latestHigh.IsZero() {
		s.latestHigh = price
	} else {
		switch s.Side {
		case types.SideTypeBuy:
			s.latestHigh = fixedpoint.Min(price, s.latestHigh)
		case types.SideTypeSell:
			s.latestHigh = fixedpoint.Max(price, s.latestHigh)
		}
	}

	if !s.activated {
		return nil
	}

	switch s.Side {
	case types.SideTypeBuy:
		s.latestHigh = fixedpoint.Min(price, s.latestHigh)

		change := price.Sub(s.latestHigh).Div(s.latestHigh)
		if change.Compare(s.CallbackRate) >= 0 {
			// submit order
			return s.triggerStop(price)
		}

	case types.SideTypeSell:
		s.latestHigh = fixedpoint.Max(price, s.latestHigh)

		change := s.latestHigh.Sub(price).Div(price)
		if change.Compare(s.CallbackRate) >= 0 {
			// submit order
			return s.triggerStop(price)
		}
	}

	return nil
}

func (s *TrailingStop2) triggerStop(price fixedpoint.Value) error {
	// reset activated flag
	defer func() {
		s.activated = false
		s.latestHigh = fixedpoint.Zero
	}()
	Notify("[TrailingStop] %s stop loss triggered. price: %f callback rate: %f", s.Symbol, price.Float64(), s.CallbackRate.Float64())
	ctx := context.Background()
	p := fixedpoint.One
	if !s.ClosePosition.IsZero() {
		p = s.ClosePosition
	}

	return s.orderExecutor.ClosePosition(ctx, p, "trailingStop")
}

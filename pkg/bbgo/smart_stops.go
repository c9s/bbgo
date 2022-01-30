package bbgo

import (
	"context"
	"errors"
	"math"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type TrailingStop struct {
	// CallbackRate is the callback rate from the previous high price
	CallbackRate fixedpoint.Value `json:"callbackRate,omitempty"`

	// ClosePosition is a percentage of the position to be closed
	ClosePosition fixedpoint.Value `json:"closePosition,omitempty"`

	// MinProfit is the percentage of the minimum profit ratio.
	// Stop order will be activiated only when the price reaches above this threshold.
	MinProfit fixedpoint.Value `json:"minProfit,omitempty"`

	// Interval is the time resolution to update the stop order
	// KLine per Interval will be used for updating the stop order
	Interval types.Interval `json:"interval,omitempty"`

	// Virtual is used when you don't want to place the real order on the exchange and lock the balance.
	// You want to handle the stop order by the strategy itself.
	Virtual bool `json:"virtual,omitempty"`
}

type TrailingStopController struct {
	*TrailingStop

	Symbol string

	position    *types.Position
	latestHigh  float64
	averageCost fixedpoint.Value
}

func NewTrailingStopController(symbol string, config *TrailingStop) *TrailingStopController {
	return &TrailingStopController{
		TrailingStop: config,
		Symbol:       symbol,
	}
}

func (c *TrailingStopController) Subscribe(session *ExchangeSession) {
	session.Subscribe(types.KLineChannel, c.Symbol, types.SubscribeOptions{
		Interval: c.Interval.String(),
	})
}

func (c *TrailingStopController) Run(ctx context.Context, session *ExchangeSession, tradeCollector *TradeCollector) {
	// store the position
	c.position = tradeCollector.Position()
	c.averageCost = c.position.AverageCost

	// Use trade collector to get the position update event
	tradeCollector.OnPositionUpdate(func(position *types.Position) {
		// update average cost if we have it.
		c.averageCost = position.AverageCost
	})

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != c.Symbol || kline.Interval != c.Interval {
			return
		}

		closePrice := kline.Close

		// update the latest high
		c.latestHigh = math.Max(closePrice, c.latestHigh)

		if c.Virtual {
			// if average cost is updated, we can check min profit
			if c.averageCost == 0 {
				return
			}

			// skip dust position
			if c.position.Base.Abs().Float64() < c.position.Market.MinQuantity || c.position.Base.Abs().Float64()*closePrice < c.position.Market.MinNotional {
				return
			}

			// if it's in the callback rate, we don't want to trigger stop
			if closePrice < c.latestHigh && changeRate(closePrice, c.latestHigh) < c.CallbackRate.Float64() {
				return
			}

			// if the profit rate is defined, and it is less than our minimum profit rate, we skip stop
			if c.MinProfit > 0 &&
				(closePrice < c.averageCost.Float64() ||
					changeRate(closePrice, c.averageCost.Float64()) < c.MinProfit.Float64()) {
				return
			}

			log.Infof("%s trailing stop emitted, latest high: %f, closed price: %f, average cost: %f, profit spread: %f",
				c.Symbol,
				c.latestHigh,
				closePrice,
				c.averageCost.Float64(),
				closePrice-c.averageCost.Float64())

			log.Infof("current %s position: %s", c.Symbol, c.position.String())

			marketOrder := c.position.NewClosePositionOrder(c.ClosePosition.Float64())
			if marketOrder != nil {
				log.Infof("submitting %s market order to stop: %+v", c.Symbol, marketOrder)

				// skip dust order
				if marketOrder.Quantity*closePrice < c.position.Market.MinNotional {
					log.Warnf("%s market order quote quantity %f < min notional %f, skip placing order", c.Symbol, marketOrder.Quantity*closePrice, c.position.Market.MinNotional)
					return
				}

				createdOrders, err := session.Exchange.SubmitOrders(ctx, *marketOrder)
				if err != nil {
					log.WithError(err).Errorf("stop market order place error")
					return
				}
				tradeCollector.OrderStore().Add(createdOrders...)
				tradeCollector.Process()

				// reset the state
				c.latestHigh = 0.0
			}
		} else {
			// place stop order only when the closed price is greater than the current average cost
			if c.position != nil && c.MinProfit > 0 && c.averageCost > 0 &&
				closePrice > c.averageCost.Float64() &&
				changeRate(closePrice, c.averageCost.Float64()) >= c.MinProfit.Float64() {

				stopPrice := c.averageCost.MulFloat64(1.0 + c.MinProfit.Float64())
				orderForm := c.GenerateStopOrder(stopPrice.Float64(), c.averageCost.Float64())
				if orderForm != nil {
					log.Infof("updating %s stop limit order to simulate trailing stop order...", c.Symbol)

					createdOrders, err := session.Exchange.SubmitOrders(ctx, *orderForm)
					if err != nil {
						log.WithError(err).Errorf("%s stop order place error", c.Symbol)
						return
					}

					tradeCollector.OrderStore().Add(createdOrders...)
					tradeCollector.Process()
				}
			}
		}
	})
}

func (c *TrailingStopController) GenerateStopOrder(stopPrice, price float64) *types.SubmitOrder {
	base := c.position.GetBase()
	if base == 0 {
		return nil
	}

	quantity := math.Abs(base.Float64())
	quoteQuantity := price * quantity

	if c.ClosePosition > 0 {
		quantity = quantity * c.ClosePosition.Float64()
	}

	// skip dust orders
	if quantity < c.position.Market.MinQuantity || quoteQuantity < c.position.Market.MinNotional {
		return nil
	}

	side := types.SideTypeSell
	if base < 0 {
		side = types.SideTypeBuy
	}

	return &types.SubmitOrder{
		Symbol:    c.Symbol,
		Market:    c.position.Market,
		Type:      types.OrderTypeStopLimit,
		Side:      side,
		StopPrice: stopPrice,
		Price:     price,
		Quantity:  quantity,
	}
}

type FixedStop struct{}

type Stop struct {
	TrailingStop *TrailingStop `json:"trailingStop,omitempty"`
	FixedStop    *FixedStop    `json:"fixedStop,omitempty"`
}

// SmartStops shares the stop order logics between different strategies
//
// See also:
// - Stop-Loss order: https://www.investopedia.com/terms/s/stop-lossorder.asp
// - Trailing Stop-loss order: https://www.investopedia.com/articles/trading/08/trailing-stop-loss.asp
//
// How to integrate this into your strategy?
//
// To use the stop controllers, you can embed this struct into your Strategy struct
//
// func (s *Strategy) Initialize() error {
//     return s.SmartStops.InitializeStopControllers(s.Symbol)
// }
// func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
//     s.SmartStops.Subscribe(session)
// }
//
// func (s *Strategy) Run() {
//     s.SmartStops.RunStopControllers(ctx, session, s.tradeCollector)
// }
//
type SmartStops struct {
	// Stops is the slice of the stop order config
	Stops []Stop `json:"stops,omitempty"`

	// StopControllers are constructed from the stop config
	StopControllers []StopController `json:"-"`
}

type StopController interface {
	Subscribe(session *ExchangeSession)
	Run(ctx context.Context, session *ExchangeSession, tradeCollector *TradeCollector)
}

func (s *SmartStops) newStopController(symbol string, config Stop) (StopController, error) {
	if config.TrailingStop != nil {
		return NewTrailingStopController(symbol, config.TrailingStop), nil
	}

	return nil, errors.New("incorrect stop controller setup")
}

func (s *SmartStops) InitializeStopControllers(symbol string) error {
	for _, stop := range s.Stops {
		controller, err := s.newStopController(symbol, stop)
		if err != nil {
			return err
		}

		s.StopControllers = append(s.StopControllers, controller)
	}
	return nil
}

func (s *SmartStops) Subscribe(session *ExchangeSession) {
	for _, stopController := range s.StopControllers {
		stopController.Subscribe(session)
	}
}

func (s *SmartStops) RunStopControllers(ctx context.Context, session *ExchangeSession, tradeCollector *TradeCollector) {
	for _, stopController := range s.StopControllers {
		stopController.Run(ctx, session, tradeCollector)
	}
}

func changeRate(a, b float64) float64 {
	return math.Abs(a-b) / b
}

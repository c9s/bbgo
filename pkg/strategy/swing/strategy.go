package swing

import (
	"context"
	"math"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	bbgo.RegisterStrategy("swing", &Strategy{})
}

type Strategy struct {
	// The notification system will be injected into the strategy automatically.
	*bbgo.Notifiability
	*bbgo.MarketDataStore
	types.Market

	// OrderExecutor is an interface for submitting order
	bbgo.OrderExecutor

	// These fields will be filled from the config file (it translates YAML to JSON)
	Symbol                string         `json:"symbol"`
	Interval              string         `json:"interval"`
	MinChange             float64        `json:"minChange"`
	BaseQuantity          float64        `json:"baseQuantity"`
	MovingAverageType     string         `json:"movingAverageType"`
	MovingAverageInterval types.Interval `json:"movingAverageInterval"`
	MovingAverageWindow   int            `json:"movingAverageWindow"`
}

type Float64Indicator interface {
	Last() float64
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	indicatorSet, ok := session.StandardIndicatorSet(s.Symbol)
	if !ok {
		return errors.Errorf("indicatorSet of %s is not configured", s.Symbol)
	}

	var inc Float64Indicator
	var iw = types.IntervalWindow{Interval: s.MovingAverageInterval, Window: s.MovingAverageWindow}

	switch s.MovingAverageType {
	case "SMA":
		inc = indicatorSet.GetSMA(iw)

	case "EWMA", "EMA":
		inc = indicatorSet.GetEWMA(iw)

	default:
		return errors.Errorf("unsupported moving average type: %s", s.MovingAverageType)

	}

	// session.Stream.OnKLineClosed
	session.Stream.OnKLineClosed(func(kline types.KLine) {
		// skip k-lines from other symbols
		if kline.Symbol != s.Symbol {
			return
		}

		movingAveragePrice := inc.Last()

		// skip it if it's near zero
		if movingAveragePrice < 0.0001 {
			return
		}

		// skip if the change is not above the minChange
		if math.Abs(kline.GetChange()) < s.MinChange {
			return
		}

		closePrice := kline.Close
		changePercentage := kline.GetChange() / kline.Open
		quantity := s.BaseQuantity * (1.0 + math.Abs(changePercentage))

		trend := kline.GetTrend()
		switch trend {
		case 1:
			// if it goes up and it's above the moving average price, then we sell
			if closePrice > movingAveragePrice {
				s.notify(":chart_with_upwards_trend: closePrice %f is above movingAveragePrice %f, submitting SELL order", closePrice, movingAveragePrice)

				_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
					Symbol:   s.Symbol,
					Market:   s.Market,
					Side:     types.SideTypeSell,
					Type:     types.OrderTypeMarket,
					Quantity: quantity,
				})
				if err != nil {
					log.WithError(err).Error("submit order error")
				}
			}
		case -1:
			// if it goes down and it's below the moving average price, then we buy
			if closePrice < movingAveragePrice {
				s.notify(":chart_with_downwards_trend: closePrice %f is below movingAveragePrice %f, submitting BUY order", closePrice, movingAveragePrice)

				_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
					Symbol:   s.Symbol,
					Market:   s.Market,
					Side:     types.SideTypeBuy,
					Type:     types.OrderTypeMarket,
					Quantity: quantity,
				})
				if err != nil {
					log.WithError(err).Error("submit order error")
				}
			}
		}
	})
	return nil
}

func (s *Strategy) notify(format string, args ...interface{}) {
	if channel, ok := s.RouteSymbol(s.Symbol); ok {
		s.NotifyTo(channel, format, args...)
	} else {
		s.Notify(format, args...)
	}
}

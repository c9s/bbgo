package swing

import (
	"context"
	"math"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	bbgo.RegisterExchangeStrategy("swing", &Strategy{})
}

type Strategy struct {
	// The notification system will be injected into the strategy automatically.
	*bbgo.Notifiability
	*bbgo.MarketDataStore
	*types.Market

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
	market, ok := session.Market(s.Symbol)
	if !ok {
		return errors.Errorf("market config of %s is not configured", s.Symbol)
	}

	marketDataStore, ok := session.MarketDataStore(s.Symbol)
	if !ok {
		return errors.Errorf("market data store of %s is not configured", s.Symbol)
	}

	indicatorSet, ok := session.StandardIndicatorSet(s.Symbol)
	if !ok {
		return errors.Errorf("indicatorSet of %s is not configured", s.Symbol)
	}

	var inc Float64Indicator
	var iw = bbgo.IntervalWindow{Interval: s.MovingAverageInterval, Window: s.MovingAverageWindow}

	switch s.MovingAverageType {
	case "SMA":
		inc, ok = indicatorSet.SMA[iw]
		if !ok {
			inc := &indicator.SMA{
				Interval: iw.Interval,
				Window:   iw.Window,
			}
			inc.BindMarketDataStore(marketDataStore)
			indicatorSet.SMA[iw] = inc
		}

	case "EWMA", "EMA":
		inc, ok = indicatorSet.EWMA[iw]
		if !ok {
			inc := &indicator.EWMA{
				Interval: iw.Interval,
				Window:   iw.Window,
			}
			inc.BindMarketDataStore(marketDataStore)
			indicatorSet.EWMA[iw] = inc
		}

	default:
		return errors.Errorf("unsupported moving average type: %s", s.MovingAverageType)

	}

	session.Stream.OnKLine(func(kline types.KLine) {
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
				s.notify(":chart_with_upwards_trend: closePrice %f is above movingAveragePrice %f, submitting sell order", closePrice, movingAveragePrice)

				_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
					Symbol:   s.Symbol,
					Market:   market,
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
				s.notify(":chart_with_downwards_trend: closePrice %f is below movingAveragePrice %f, submitting buy order", closePrice, movingAveragePrice)

				_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
					Symbol:   s.Symbol,
					Market:   market,
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

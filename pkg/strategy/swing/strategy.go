package swing

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "swing"

// Float64Indicator is the indicators (SMA and EWMA) that we want to use are returning float64 data.
type Float64Indicator interface {
	Last() float64
}

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	// OrderExecutor is an interface for submitting order.
	// This field will be injected automatically since it's a single exchange strategy.
	bbgo.OrderExecutor

	// if Symbol string field is defined, bbgo will know it's a symbol-based strategy
	// The following embedded fields will be injected with the corresponding instances.

	// MarketDataStore is a pointer only injection field. public trades, k-lines (candlestick)
	// and order book updates are maintained in the market data store.
	// This field will be injected automatically since we defined the Symbol field.
	*bbgo.MarketDataStore

	// StandardIndicatorSet contains the standard indicators of a market (symbol)
	// This field will be injected automatically since we defined the Symbol field.
	*bbgo.StandardIndicatorSet

	// Market stores the configuration of the market, for example, VolumePrecision, PricePrecision, MinLotSize... etc
	// This field will be injected automatically since we defined the Symbol field.
	types.Market

	// These fields will be filled from the config file (it translates YAML to JSON)
	Symbol string `json:"symbol"`

	// Interval is the interval of the kline channel we want to subscribe,
	// the kline event will trigger the strategy to check if we need to submit order.
	Interval types.Interval `json:"interval"`

	// MinChange filters out the k-lines with small changes. so that our strategy will only be triggered
	// in specific events.
	MinChange fixedpoint.Value `json:"minChange"`

	// BaseQuantity is the base quantity of the submit order. for both BUY and SELL, market order will be used.
	BaseQuantity fixedpoint.Value `json:"baseQuantity"`

	// MovingAverageType is the moving average indicator type that we want to use,
	// it could be SMA or EWMA
	MovingAverageType string `json:"movingAverageType"`

	// MovingAverageInterval is the interval of k-lines for the moving average indicator to calculate,
	// it could be "1m", "5m", "1h" and so on.  note that, the moving averages are calculated from
	// the k-line data we subscribed
	MovingAverageInterval types.Interval `json:"movingAverageInterval"`

	// MovingAverageWindow is the number of the window size of the moving average indicator.
	// The number of k-lines in the window. generally used window sizes are 7, 25 and 99 in the TradingView.
	MovingAverageWindow int `json:"movingAverageWindow"`
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	var inc Float64Indicator
	var iw = types.IntervalWindow{Interval: s.MovingAverageInterval, Window: s.MovingAverageWindow}

	switch s.MovingAverageType {
	case "SMA":
		inc = s.StandardIndicatorSet.SMA(iw)

	case "EWMA", "EMA":
		inc = s.StandardIndicatorSet.EWMA(iw)

	default:
		return fmt.Errorf("unsupported moving average type: %s", s.MovingAverageType)

	}

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
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
		if kline.GetChange().Abs().Compare(s.MinChange) < 0 {
			return
		}

		closePrice := kline.Close
		changePercentage := kline.GetChange().Div(kline.Open)
		quantity := s.BaseQuantity.Mul(fixedpoint.One.Add(changePercentage.Abs()))

		trend := kline.Direction()
		switch trend {
		case types.DirectionUp:
			// if it goes up and it's above the moving average price, then we sell
			if closePrice.Float64() > movingAveragePrice {
				s.notify(":chart_with_upwards_trend: closePrice %v is above movingAveragePrice %v, submitting SELL order", closePrice, movingAveragePrice)

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
		case types.DirectionDown:
			// if it goes down and it's below the moving average price, then we buy
			if closePrice.Float64() < movingAveragePrice {
				s.notify(":chart_with_downwards_trend: closePrice %v is below movingAveragePrice %v, submitting BUY order", closePrice, movingAveragePrice)

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
	if channel, ok := bbgo.Notification.RouteSymbol(s.Symbol); ok {
		bbgo.NotifyTo(channel, format, args...)
	} else {
		bbgo.Notify(format, args...)
	}
}

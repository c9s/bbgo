package factorzoo

import (
	"context"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/strategy/factorzoo/factors"
	"github.com/c9s/bbgo/pkg/types"
)

type Linear struct {
	Symbol string
	Market types.Market `json:"-"`
	types.IntervalWindow

	// MarketOrder is the option to enable market order short.
	MarketOrder bool `json:"marketOrder"`

	Quantity     fixedpoint.Value      `json:"quantity"`
	StopEMARange fixedpoint.Value      `json:"stopEMARange"`
	StopEMA      *types.IntervalWindow `json:"stopEMA"`

	// Xs (input), factors & indicators
	divergence *factorzoo.PVD           // price volume divergence
	reversion  *factorzoo.PMR           // price mean reversion
	momentum   *factorzoo.MOM           // price momentum from paper, alpha 101
	drift      *indicatorv2.DriftStream // GBM
	volume     *factorzoo.VMOM          // quarterly volume momentum

	// Y (output), internal rate of return
	irr *factorzoo.RR

	orderExecutor *bbgo.GeneralOrderExecutor
	session       *bbgo.ExchangeSession
	activeOrders  *bbgo.ActiveOrderBook

	bbgo.QuantityOrAmount
}

func (s *Linear) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Linear) Bind(session *bbgo.ExchangeSession, orderExecutor *bbgo.GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	position := orderExecutor.Position()
	symbol := position.Symbol

	// initialize factor indicators
	//
	// All the factors must be created before registering the kline handler below,
	// so that they are updated before the handler reads them.
	// The factors only use the kline passed to their subscriber, so they can subscribe
	// to the shared kline stream directly: AddSubscriber replays the kline history to them.
	indicators := session.Indicators(symbol)
	kLines := indicators.KLines(s.Interval)
	s.divergence = factorzoo.NewPVD(kLines, 60)
	s.reversion = factorzoo.NewPMR(kLines, 60)
	s.drift = indicatorv2.Drift(indicators.CLOSE(s.Interval), 7)
	s.momentum = factorzoo.NewMOM(kLines, 1)
	s.volume = factorzoo.NewVMOM(kLines, 90)

	s.irr = factorzoo.NewRR(kLines, 2)

	predLst := types.NewQueue(s.Window)
	session.MarketDataStream.OnKLineClosed(types.KLineWith(symbol, s.Interval, func(kline types.KLine) {

		ctx := context.Background()

		// graceful cancel all active orders
		_ = orderExecutor.GracefulCancel(ctx)

		// take past window days' values to predict future return
		// (e.g., 5 here in default configuration file)
		a := []floats.Slice{
			s.divergence.Slice[len(s.divergence.Slice)-s.Window-2 : len(s.divergence.Slice)-2],
			s.reversion.Slice[len(s.reversion.Slice)-s.Window-2 : len(s.reversion.Slice)-2],
			s.drift.Slice[len(s.drift.Slice)-s.Window-2 : len(s.drift.Slice)-2],
			s.momentum.Slice[len(s.momentum.Slice)-s.Window-2 : len(s.momentum.Slice)-2],
			s.volume.Slice[len(s.volume.Slice)-s.Window-2 : len(s.volume.Slice)-2],
		}
		// e.g., s.window is 5
		// factors array from day -4 to day 0, [[0.1, 0.2, 0.35, 0.3 , 0.25], [1.1, -0.2, 1.35, -0.3 , -0.25], ...]
		// the binary(+/-) daily return rate from day -3 to day 1, [0, 1, 1, 0, 0]
		// then we take the latest available factors array into linear regression model
		b := []floats.Slice{filter(s.irr.Slice[len(s.irr.Slice)-s.Window-1:len(s.irr.Slice)-1], binary)}
		var x []types.Series
		var y []types.Series

		x = append(x, &a[0])
		x = append(x, &a[1])
		x = append(x, &a[2])
		x = append(x, &a[3])
		x = append(x, &a[4])
		//x = append(x, &a[5])

		y = append(y, &b[0])
		model := types.LogisticRegression(x, y[0], s.Window, 8000, 0.0001)

		// use the last value from indicators, or the SeriesExtends' predict function. (e.g., look back: 5)
		input := []float64{
			s.divergence.Last(0),
			s.reversion.Last(0),
			s.drift.Last(0),
			s.momentum.Last(0),
			s.volume.Last(0),
		}
		pred := model.Predict(input)
		predLst.Update(pred)

		qty := s.Quantity //s.QuantityOrAmount.CalculateQuantity(kline.Close)

		// the scale of pred is from 0.0 to 1.0
		// 0.5 can be used as the threshold
		// we use the time-series rolling prediction values here
		if pred > predLst.Mean() {
			if position.IsShort() {
				s.ClosePosition(ctx, one)
				s.placeMarketOrder(ctx, types.SideTypeBuy, qty, symbol)
			} else if position.IsClosed() {
				s.placeMarketOrder(ctx, types.SideTypeBuy, qty, symbol)
			}
		} else if pred < predLst.Mean() {
			if position.IsLong() {
				s.ClosePosition(ctx, one)
				s.placeMarketOrder(ctx, types.SideTypeSell, qty, symbol)
			} else if position.IsClosed() {
				s.placeMarketOrder(ctx, types.SideTypeSell, qty, symbol)
			}
		}
		// pass if position is opened and not dust, and remain the same direction with alpha signal

		// alpha-weighted inventory and cash
		//alpha := fixedpoint.NewFromFloat(s.r1.Last())
		//targetBase := s.QuantityOrAmount.CalculateQuantity(kline.Close).Mul(alpha)
		////s.ClosePosition(ctx, one)
		//diffQty := targetBase.Sub(position.Base)
		//log.Info(alpha.Float64(), position.Base, diffQty.Float64())
		//
		//if diffQty.Sign() > 0 {
		//	s.placeMarketOrder(ctx, types.SideTypeBuy, diffQty.Abs(), symbol)
		//} else if diffQty.Sign() < 0 {
		//	s.placeMarketOrder(ctx, types.SideTypeSell, diffQty.Abs(), symbol)
		//}
	}))

	if !bbgo.IsBackTesting {
		session.MarketDataStream.OnMarketTrade(func(trade types.Trade) {
		})
	}
}

func (s *Linear) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	return s.orderExecutor.ClosePosition(ctx, percentage)
}

func (s *Linear) placeMarketOrder(ctx context.Context, side types.SideType, quantity fixedpoint.Value, symbol string) {
	market, _ := s.session.Market(symbol)
	_, err := s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Symbol:   symbol,
		Market:   market,
		Side:     side,
		Type:     types.OrderTypeMarket,
		Quantity: quantity,
		//TimeInForce: types.TimeInForceGTC,
		Tag: "linear",
	})
	if err != nil {
		log.WithError(err).Errorf("can not place market order")
	}
}

func binary(val float64) float64 {
	if val > 0. {
		return 1.
	} else {
		return 0.
	}
}

func filter(data []float64, f func(float64) float64) []float64 {
	fltd := make([]float64, 0)
	for _, e := range data {
		//if f(e) >= 0. {
		fltd = append(fltd, f(e))
		//}
	}
	return fltd
}

package factorzoo

import (
	"context"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
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
	divergence *factorzoo.AVD // amplitude volume divergence
	reversion  *factorzoo.PMR // price mean reversion
	momentum   *factorzoo.MOM // price momentum from WorldQuant's paper, alpha 101
	drift      *indicator.Drift
	volume     *factorzoo.VMOM  // quarterly volume momentum
	bars       *factorzoo.LSBAR // long short bar accumulation

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
	store, _ := session.MarketDataStore(symbol)

	s.divergence = &factorzoo.AVD{IntervalWindow: types.IntervalWindow{Window: s.Window, Interval: s.Interval}}
	s.divergence.Bind(store)
	preloadDivergence(s.divergence, store)
	s.reversion = &factorzoo.PMR{IntervalWindow: types.IntervalWindow{Window: s.Window, Interval: s.Interval}}
	s.reversion.Bind(store)
	preloadReversion(s.reversion, store)
	s.drift = &indicator.Drift{IntervalWindow: types.IntervalWindow{Window: 5, Interval: s.Interval}}
	s.drift.Bind(store)
	preloadDrift(s.drift, store)
	s.momentum = &factorzoo.MOM{IntervalWindow: types.IntervalWindow{Window: 1, Interval: s.Interval}}
	s.momentum.Bind(store)
	preloadMomentum(s.momentum, store)
	s.volume = &factorzoo.VMOM{IntervalWindow: types.IntervalWindow{Window: 90, Interval: s.Interval}}
	s.volume.Bind(store)
	preloadVolume(s.volume, store)
	s.bars = &factorzoo.LSBAR{IntervalWindow: types.IntervalWindow{Window: 360, Interval: s.Interval}}
	s.bars.Bind(store)
	preloadBars(s.bars, store)

	s.irr = &factorzoo.RR{IntervalWindow: types.IntervalWindow{Window: 2, Interval: s.Interval}}
	s.irr.Bind(store)
	preloadIRR(s.irr, store)
	session.MarketDataStream.OnKLineClosed(types.KLineWith(symbol, s.Interval, func(kline types.KLine) {

		ctx := context.Background()

		// graceful cancel all active orders
		_ = orderExecutor.GracefulCancel(ctx)

		a := []types.Float64Slice{s.divergence.Values[len(s.divergence.Values)-s.Window : len(s.divergence.Values)-2],
			s.reversion.Values[len(s.reversion.Values)-s.Window : len(s.reversion.Values)-2],
			s.drift.Values[len(s.drift.Values)-s.Window : len(s.drift.Values)-2],
			s.momentum.Values[len(s.momentum.Values)-s.Window : len(s.momentum.Values)-2],
			s.volume.Values[len(s.volume.Values)-s.Window : len(s.volume.Values)-2],
			//s.bars.Values[len(s.bars.Values)-s.Window : len(s.bars.Values)-2],
		}
		b := []types.Float64Slice{filter(s.irr.Values[len(s.irr.Values)-(s.Window-1):len(s.irr.Values)-(2-1)], binary)}
		var x []types.Series
		var y []types.Series

		x = append(x, &a[0])
		x = append(x, &a[1])
		x = append(x, &a[2])
		x = append(x, &a[3])
		x = append(x, &a[4])
		//x = append(x, &a[5])

		y = append(y, &b[0])
		//log.Infof("actual: %f", y[0])

		model := types.LogisticRegression(x, y[0], s.Window, 8000, 0.0001)

		input := []float64{
			s.divergence.Predict(5, 20),
			s.reversion.Predict(5, 20),
			s.drift.Predict(5, 20),
			s.momentum.Predict(5, 20),
			s.volume.Predict(5, 20),
			//s.bars.Predict(5, 20),
		}
		//log.Info(input)
		pred := model.Predict(input)
		//log.Infof("prediction: %f", pred)
		//qty := s.QuantityOrAmount.CalculateQuantity(kline.Close)
		if pred > 0.5 {
			if position.IsShort() && !position.IsDust(kline.Close) {
				s.ClosePosition(ctx, one)
				s.placeMarketOrder(ctx, types.SideTypeBuy, s.Quantity, symbol)
			} else if position.IsClosed() || position.IsDust(kline.Close) {
				s.placeMarketOrder(ctx, types.SideTypeBuy, s.Quantity, symbol)
			}
		} else if pred < 0.5 {
			if position.IsLong() && !position.IsDust(kline.Close) {
				s.ClosePosition(ctx, one)
				s.placeMarketOrder(ctx, types.SideTypeSell, s.Quantity, symbol)
			} else if position.IsClosed() || position.IsDust(kline.Close) {
				s.placeMarketOrder(ctx, types.SideTypeSell, s.Quantity, symbol)
			}
		}
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

func (s *Linear) placeLimitOrder(ctx context.Context, side types.SideType, quantity fixedpoint.Value, price fixedpoint.Value, symbol string) {
	market, _ := s.session.Market(symbol)
	_, err := s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Symbol:   symbol,
		Market:   market,
		Side:     side,
		Type:     types.OrderTypeLimitMaker,
		Quantity: quantity,
		Price:    price,
		//TimeInForce: types.TimeInForceGTC,
		Tag: "linearLimit",
	})
	if err != nil {
		log.WithError(err).Errorf("can not place market order")
	}
}

func preloadDivergence(divergence *factorzoo.AVD, store *bbgo.MarketDataStore) {
	klines, _ := store.KLinesOfInterval(divergence.Interval)

	log.Debugf("updating divergence indicator: %d klines", len(*klines))

	for i := 0; i < len(*klines); i++ {
		divergence.Update(*klines)
	}
}

func preloadReversion(reversion *factorzoo.PMR, store *bbgo.MarketDataStore) {
	klines, _ := store.KLinesOfInterval(reversion.Interval)

	log.Debugf("updating reversion indicator: %d klines", len(*klines))

	for i := 0; i < len(*klines); i++ {
		reversion.Update(*klines)
	}
}

func preloadDrift(drift *indicator.Drift, store *bbgo.MarketDataStore) {
	klines, _ := store.KLinesOfInterval(drift.Interval)

	log.Debugf("updating drift indicator: %d klines", len(*klines))

	for i := 0; i < len(*klines); i++ {
		drift.CalculateAndUpdate(*klines)
	}
}

func preloadMomentum(momentum *factorzoo.MOM, store *bbgo.MarketDataStore) {
	klines, _ := store.KLinesOfInterval(momentum.Interval)

	log.Debugf("updating momentum indicator: %d klines", len(*klines))

	for i := 0; i < len(*klines); i++ {
		momentum.Update(*klines)
	}
}

func preloadMomentum2(momentum *factorzoo.MOM2, store *bbgo.MarketDataStore) {
	klines, _ := store.KLinesOfInterval(momentum.Interval)

	log.Debugf("updating momentum2 indicator: %d klines", len(*klines))

	for i := 0; i < len(*klines); i++ {
		momentum.Update(*klines)
	}
}

func preloadVolume(momentum *factorzoo.VMOM, store *bbgo.MarketDataStore) {
	klines, _ := store.KLinesOfInterval(momentum.Interval)

	log.Debugf("updating volume momentum indicator: %d klines", len(*klines))

	for i := 0; i < len(*klines); i++ {
		momentum.Update(*klines)
	}
}

func preloadBars(bars *factorzoo.LSBAR, store *bbgo.MarketDataStore) {
	klines, _ := store.KLinesOfInterval(bars.Interval)

	log.Debugf("updating long short bars indicator: %d klines", len(*klines))

	for i := 0; i < len(*klines); i++ {
		bars.Update(*klines)
	}
}

func preloadIRR(irr *factorzoo.RR, store *bbgo.MarketDataStore) {
	klines, _ := store.KLinesOfInterval(irr.Interval)

	log.Debugf("updating irr indicator: %d klines", len(*klines))

	for i := 0; i < len(*klines); i++ {
		irr.CalculateAndUpdate(*klines)
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

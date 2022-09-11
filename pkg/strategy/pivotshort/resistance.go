package pivotshort

import (
	"context"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

type ResistanceShort struct {
	Enabled bool         `json:"enabled"`
	Symbol  string       `json:"-"`
	Market  types.Market `json:"-"`

	types.IntervalWindow

	MinDistance   fixedpoint.Value `json:"minDistance"`
	GroupDistance fixedpoint.Value `json:"groupDistance"`
	NumLayers     int              `json:"numLayers"`
	LayerSpread   fixedpoint.Value `json:"layerSpread"`
	Quantity      fixedpoint.Value `json:"quantity"`
	Leverage      fixedpoint.Value `json:"leverage"`
	Ratio         fixedpoint.Value `json:"ratio"`

	TrendEMA *bbgo.TrendEMA `json:"trendEMA"`

	session       *bbgo.ExchangeSession
	orderExecutor *bbgo.GeneralOrderExecutor

	resistancePivot        *indicator.PivotLow
	resistancePrices       []float64
	currentResistancePrice fixedpoint.Value

	activeOrders *bbgo.ActiveOrderBook

	// StrategyController
	bbgo.StrategyController
}

func (s *ResistanceShort) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})

	if s.TrendEMA != nil {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.TrendEMA.Interval})
	}
}

func (s *ResistanceShort) Bind(session *bbgo.ExchangeSession, orderExecutor *bbgo.GeneralOrderExecutor) {
	if s.GroupDistance.IsZero() {
		s.GroupDistance = fixedpoint.NewFromFloat(0.01)
	}

	s.session = session
	s.orderExecutor = orderExecutor
	s.activeOrders = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeOrders.OnFilled(func(o types.Order) {
		// reset resistance price
		s.currentResistancePrice = fixedpoint.Zero
	})
	s.activeOrders.BindStream(session.UserDataStream)

	// StrategyController
	s.Status = types.StrategyStatusRunning

	if s.TrendEMA != nil {
		s.TrendEMA.Bind(session, orderExecutor)
	}

	s.resistancePivot = session.StandardIndicatorSet(s.Symbol).PivotLow(s.IntervalWindow)

	// use the last kline from the history before we get the next closed kline
	s.updateResistanceOrders(fixedpoint.NewFromFloat(s.resistancePivot.Last()))

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {
		// StrategyController
		if s.Status != types.StrategyStatusRunning {
			return
		}

		// trend EMA protection
		if s.TrendEMA != nil && !s.TrendEMA.GradientAllowed() {
			return
		}

		position := s.orderExecutor.Position()
		if position.IsOpened(kline.Close) {
			return
		}

		s.updateResistanceOrders(kline.Close)
	}))
}

// updateCurrentResistancePrice updates the current resistance price
// we should only update the resistance price when:
// 1) the close price is already above the current resistance price by (1 + minDistance)
// 2) the next resistance price is lower than the current resistance price.
func (s *ResistanceShort) updateCurrentResistancePrice(closePrice fixedpoint.Value) bool {
	minDistance := s.MinDistance.Float64()
	groupDistance := s.GroupDistance.Float64()
	resistancePrices := findPossibleResistancePrices(closePrice.Float64()*(1.0+minDistance), groupDistance, s.resistancePivot.Values.Tail(6))
	if len(resistancePrices) == 0 {
		return false
	}

	log.Infof("%s close price: %f, min distance: %f, possible resistance prices: %+v", s.Symbol, closePrice.Float64(), minDistance, resistancePrices)

	nextResistancePrice := fixedpoint.NewFromFloat(resistancePrices[0])

	if s.currentResistancePrice.IsZero() {
		s.currentResistancePrice = nextResistancePrice
		return true
	}

	// if the current sell price is out-dated
	// or
	// the next resistance is lower than the current one.
	minPriceToUpdate := s.currentResistancePrice.Mul(one.Add(s.MinDistance))
	if closePrice.Compare(minPriceToUpdate) > 0 || nextResistancePrice.Compare(s.currentResistancePrice) < 0 {
		s.currentResistancePrice = nextResistancePrice
		return true
	}

	return false
}

func (s *ResistanceShort) updateResistanceOrders(closePrice fixedpoint.Value) {
	ctx := context.Background()
	resistanceUpdated := s.updateCurrentResistancePrice(closePrice)
	if resistanceUpdated {
		s.placeResistanceOrders(ctx, s.currentResistancePrice)
	} else if s.activeOrders.NumOfOrders() == 0 && !s.currentResistancePrice.IsZero() {
		s.placeResistanceOrders(ctx, s.currentResistancePrice)
	}
}

func (s *ResistanceShort) placeResistanceOrders(ctx context.Context, resistancePrice fixedpoint.Value) {
	totalQuantity, err := bbgo.CalculateBaseQuantity(s.session, s.Market, resistancePrice, s.Quantity, s.Leverage)
	if err != nil {
		log.WithError(err).Errorf("quantity calculation error")
	}

	if totalQuantity.IsZero() {
		return
	}

	bbgo.Notify("Next %s resistance price at %f, updating resistance orders with total quantity %f", s.Symbol, s.currentResistancePrice.Float64(), totalQuantity.Float64())

	numLayers := s.NumLayers
	if numLayers == 0 {
		numLayers = 1
	}

	numLayersF := fixedpoint.NewFromInt(int64(numLayers))
	layerSpread := s.LayerSpread
	quantity := totalQuantity.Div(numLayersF)

	if s.activeOrders.NumOfOrders() > 0 {
		if err := s.orderExecutor.GracefulCancelActiveOrderBook(ctx, s.activeOrders); err != nil {
			log.WithError(err).Errorf("can not cancel resistance orders: %+v", s.activeOrders.Orders())
		}
	}

	log.Infof("placing resistance orders: resistance price = %f, layer quantity = %f, num of layers = %d", resistancePrice.Float64(), quantity.Float64(), numLayers)

	var sellPriceStart = resistancePrice.Mul(fixedpoint.One.Add(s.Ratio))
	var orderForms []types.SubmitOrder
	for i := 0; i < numLayers; i++ {
		balances := s.session.GetAccount().Balances()
		quoteBalance := balances[s.Market.QuoteCurrency]
		baseBalance := balances[s.Market.BaseCurrency]
		_ = quoteBalance
		_ = baseBalance

		spread := layerSpread.Mul(fixedpoint.NewFromInt(int64(i)))
		price := sellPriceStart.Mul(one.Add(spread))
		log.Infof("resistance sell price = %f", price.Float64())
		log.Infof("placing resistance short order #%d: price = %f, quantity = %f", i, price.Float64(), quantity.Float64())

		orderForms = append(orderForms, types.SubmitOrder{
			Symbol:           s.Symbol,
			Side:             types.SideTypeSell,
			Type:             types.OrderTypeLimitMaker,
			Price:            price,
			Quantity:         quantity,
			Tag:              "resistanceShort",
			MarginSideEffect: types.SideEffectTypeMarginBuy,
		})
	}

	createdOrders, err := s.orderExecutor.SubmitOrders(ctx, orderForms...)
	if err != nil {
		log.WithError(err).Errorf("can not place resistance order")
	}
	s.activeOrders.Add(createdOrders...)
}

func findPossibleSupportPrices(closePrice float64, groupDistance float64, lows []float64) []float64 {
	return floats.Group(floats.Lower(lows, closePrice), groupDistance)
}

func findPossibleResistancePrices(closePrice float64, groupDistance float64, lows []float64) []float64 {
	return floats.Group(floats.Higher(lows, closePrice), groupDistance)
}

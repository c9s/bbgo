package pivotshort

import (
	"context"
	"sort"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

type ResistanceShort struct {
	Enabled bool         `json:"enabled"`
	Symbol  string       `json:"-"`
	Market  types.Market `json:"-"`

	types.IntervalWindow

	MinDistance fixedpoint.Value `json:"minDistance"`
	NumLayers   int              `json:"numLayers"`
	LayerSpread fixedpoint.Value `json:"layerSpread"`
	Quantity    fixedpoint.Value `json:"quantity"`
	Ratio       fixedpoint.Value `json:"ratio"`

	session       *bbgo.ExchangeSession
	orderExecutor *bbgo.GeneralOrderExecutor

	resistancePivot     *indicator.Pivot
	resistancePrices    []float64
	nextResistancePrice fixedpoint.Value

	activeOrders *bbgo.ActiveOrderBook
}

func (s *ResistanceShort) Bind(session *bbgo.ExchangeSession, orderExecutor *bbgo.GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor
	s.activeOrders = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeOrders.BindStream(session.UserDataStream)

	store, _ := session.MarketDataStore(s.Symbol)

	s.resistancePivot = &indicator.Pivot{IntervalWindow: s.IntervalWindow}
	s.resistancePivot.Bind(store)

	// preload history kline data to the resistance pivot indicator
	// we use the last kline to find the higher lows
	lastKLine := preloadPivot(s.resistancePivot, store)

	// use the last kline from the history before we get the next closed kline
	if lastKLine != nil {
		s.findNextResistancePriceAndPlaceOrders(lastKLine.Close)
	}

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {
		position := s.orderExecutor.Position()
		if position.IsOpened(kline.Close) {
			return
		}

		s.findNextResistancePriceAndPlaceOrders(kline.Close)
	}))
}

func (s *ResistanceShort) findNextResistancePriceAndPlaceOrders(closePrice fixedpoint.Value) {
	// if the close price is still lower than the resistance price, then we don't have to update
	if closePrice.Compare(s.nextResistancePrice) <= 0 {
		return
	}

	minDistance := s.MinDistance.Float64()
	lows := s.resistancePivot.Lows
	resistancePrices := findPossibleResistancePrices(closePrice.Float64(), minDistance, lows)

	log.Infof("last price: %f, possible resistance prices: %+v", closePrice.Float64(), resistancePrices)

	ctx := context.Background()
	if len(resistancePrices) > 0 {
		nextResistancePrice := fixedpoint.NewFromFloat(resistancePrices[0])
		if nextResistancePrice.Compare(s.nextResistancePrice) != 0 {
			bbgo.Notify("Found next resistance price: %f", nextResistancePrice.Float64())
			s.nextResistancePrice = nextResistancePrice
			s.placeResistanceOrders(ctx, nextResistancePrice)
		}
	}
}

func (s *ResistanceShort) placeResistanceOrders(ctx context.Context, resistancePrice fixedpoint.Value) {
	futuresMode := s.session.Futures || s.session.IsolatedFutures
	_ = futuresMode

	totalQuantity := s.Quantity
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

	var orderForms []types.SubmitOrder
	for i := 0; i < numLayers; i++ {
		balances := s.session.GetAccount().Balances()
		quoteBalance := balances[s.Market.QuoteCurrency]
		baseBalance := balances[s.Market.BaseCurrency]
		_ = quoteBalance
		_ = baseBalance

		// price = (resistance_price * (1.0 + ratio)) * ((1.0 + layerSpread) * i)
		price := resistancePrice.Mul(fixedpoint.One.Add(s.Ratio))
		spread := layerSpread.Mul(fixedpoint.NewFromInt(int64(i)))
		price = price.Add(spread)
		log.Infof("price = %f", price.Float64())

		log.Infof("placing bounce short order #%d: price = %f, quantity = %f", i, price.Float64(), quantity.Float64())

		orderForms = append(orderForms, types.SubmitOrder{
			Symbol:           s.Symbol,
			Side:             types.SideTypeSell,
			Type:             types.OrderTypeLimitMaker,
			Price:            price,
			Quantity:         quantity,
			Tag:              "resistanceShort",
			MarginSideEffect: types.SideEffectTypeMarginBuy,
		})

		// TODO: fix futures mode later
		/*
			if futuresMode {
				if quantity.Mul(price).Compare(quoteBalance.Available) <= 0 {
				}
			}
		*/
	}

	createdOrders, err := s.orderExecutor.SubmitOrders(ctx, orderForms...)
	if err != nil {
		log.WithError(err).Errorf("can not place resistance order")
	}
	s.activeOrders.Add(createdOrders...)
}

func findPossibleSupportPrices(closePrice float64, minDistance float64, lows []float64) []float64 {
	// sort float64 in increasing order
	// lower to higher prices
	sort.Float64s(lows)

	var supportPrices []float64
	var last = closePrice
	for i := len(lows) - 1; i >= 0; i-- {
		price := lows[i]

		// filter prices that are lower than the current closed price
		if price > closePrice {
			continue
		}

		if (price / last) < (1.0 - minDistance) {
			continue
		}

		supportPrices = append(supportPrices, price)
		last = price
	}

	return supportPrices
}

func findPossibleResistancePrices(closePrice float64, minDistance float64, lows []float64) []float64 {
	// sort float64 in increasing order
	// lower to higher prices
	sort.Float64s(lows)

	var resistancePrices []float64
	var last = closePrice
	for _, price := range lows {
		// filter prices that are lower than the current closed price
		if price < closePrice {
			continue
		}

		if (price / last) < (1.0 + minDistance) {
			continue
		}

		resistancePrices = append(resistancePrices, price)
		last = price
	}

	return resistancePrices
}

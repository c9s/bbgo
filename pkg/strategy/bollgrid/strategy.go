package bollgrid

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

var log = logrus.WithField("strategy", "bollgrid")

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy("bollgrid", &Strategy{})
}

type Strategy struct {
	// The notification system will be injected into the strategy automatically.
	// This field will be injected automatically since it's a single exchange strategy.
	*bbgo.Notifiability

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

	// Graceful let you define the graceful shutdown handler
	*bbgo.Graceful

	// Market stores the configuration of the market, for example, VolumePrecision, PricePrecision, MinLotSize... etc
	// This field will be injected automatically since we defined the Symbol field.
	types.Market

	// These fields will be filled from the config file (it translates YAML to JSON)
	Symbol string `json:"symbol"`

	// Interval is the interval used by the BOLLINGER indicator (which uses K-Line as its source price)
	Interval types.Interval `json:"interval"`

	// RepostInterval is the interval for re-posting maker orders
	RepostInterval types.Interval `json:"repostInterval"`

	// GridPips is the pips of grid
	// e.g., 0.001, so that your orders will be submitted at price like 0.127, 0.128, 0.129, 0.130
	GridPips fixedpoint.Value `json:"gridPips"`

	ProfitSpread fixedpoint.Value `json:"profitSpread"`

	// GridNum is the grid number, how many orders you want to post on the orderbook.
	GridNum int `json:"gridNumber"`

	// Quantity is the quantity you want to submit for each order.
	Quantity float64 `json:"quantity"`

	// activeOrders is the locally maintained active order book of the maker orders.
	activeOrders *bbgo.LocalActiveOrderBook

	orders *bbgo.OrderStore

	// boll is the BOLLINGER indicator we used for predicting the price.
	boll *indicator.BOLL
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// currently we need the 1m kline to update the last close price and indicators
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval.String()})
}

func (s *Strategy) updateBidOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	quoteCurrency := s.Market.QuoteCurrency
	balances := session.Account.Balances()

	balance, ok := balances[quoteCurrency]
	if !ok || balance.Available <= 0 {
		return
	}

	var downBand = s.boll.LastDownBand()
	if downBand <= 0.0 {
		return
	}

	var startPrice = downBand

	var submitOrders []types.SubmitOrder
	for i := 0; i < s.GridNum; i++ {
		submitOrders = append(submitOrders, types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        types.SideTypeBuy,
			Type:        types.OrderTypeLimit,
			Market:      s.Market,
			Quantity:    s.Quantity,
			Price:       startPrice,
			TimeInForce: "GTC",
		})

		startPrice -= s.GridPips.Float64()
	}

	orders, err := orderExecutor.SubmitOrders(context.Background(), submitOrders...)
	if err != nil {
		log.WithError(err).Errorf("can not place orders")
		return
	}

	s.activeOrders.Add(orders...)
	s.orders.Add(orders...)
}

func (s *Strategy) updateAskOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	baseCurrency := s.Market.BaseCurrency
	balances := session.Account.Balances()

	balance, ok := balances[baseCurrency]
	if !ok || balance.Available <= 0 {
		return
	}

	var upBand = s.boll.LastUpBand()
	if upBand <= 0.0 {
		return
	}

	var startPrice = upBand

	var submitOrders []types.SubmitOrder
	for i := 0; i < s.GridNum; i++ {
		submitOrders = append(submitOrders, types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        types.SideTypeSell,
			Type:        types.OrderTypeLimit,
			Market:      s.Market,
			Quantity:    s.Quantity,
			Price:       startPrice,
			TimeInForce: "GTC",
		})

		startPrice += s.GridPips.Float64()
	}

	orders, err := orderExecutor.SubmitOrders(context.Background(), submitOrders...)
	if err != nil {
		log.WithError(err).Errorf("can not place orders")
		return
	}

	s.orders.Add(orders...)
	s.activeOrders.Add(orders...)
}

func (s *Strategy) placeGridOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	quoteCurrency := s.Market.QuoteCurrency
	balances := session.Account.Balances()

	balance, ok := balances[quoteCurrency]
	if !ok || balance.Available <= 0 {
		return
	}

	var upBand = s.boll.LastUpBand()
	if upBand <= 0.0 {
		return
	}

	var downBand = s.boll.LastDownBand()
	if downBand <= 0.0 {
		return
	}

	currentPrice, ok := session.LastPrice(s.Symbol)
	if !ok {
		return
	}
	if currentPrice > upBand || currentPrice < downBand {
		return
	}

	ema99 := s.StandardIndicatorSet.GetEWMA(types.IntervalWindow{Interval: s.Interval, Window: 99})
	ema25 := s.StandardIndicatorSet.GetEWMA(types.IntervalWindow{Interval: s.Interval, Window: 25})
	ema7 := s.StandardIndicatorSet.GetEWMA(types.IntervalWindow{Interval: s.Interval, Window: 7})

	priceRange := upBand - downBand
	gridSize := priceRange / float64(s.GridNum)

	var orders []types.SubmitOrder
	for price := downBand; price <= upBand; price += gridSize {
		var side types.SideType
		if price > currentPrice {
			side = types.SideTypeSell
		} else {
			side = types.SideTypeBuy
		}

		// trend up
		switch side {

		case types.SideTypeBuy:
			if ema7.Last() > ema25.Last()*1.001 && ema25.Last() > ema99.Last()*1.0005 {
				log.Infof("all ema lines trend up, skip buy")
				continue
			}

		case types.SideTypeSell:
			if ema7.Last() < ema25.Last()*(1-0.004) && ema25.Last() < ema99.Last()*(1-0.0005) {
				log.Infof("all ema lines trend down, skip sell")
				continue
			}
		}

		order := types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        side,
			Type:        types.OrderTypeLimit,
			Market:      s.Market,
			Quantity:    s.Quantity,
			Price:       price,
			TimeInForce: "GTC",
		}
		log.Infof("submitting order: %s", order.String())
		orders = append(orders, order)
	}

	createdOrders, err := orderExecutor.SubmitOrders(context.Background(), orders...)
	if err != nil {
		log.WithError(err).Errorf("can not place orders")
		return
	}

	s.activeOrders.Add(createdOrders...)
	s.orders.Add(createdOrders...)
}

func (s *Strategy) updateOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {

	if err := session.Exchange.CancelOrders(context.Background(), s.activeOrders.Orders()...); err != nil {
		log.WithError(err).Errorf("cancel order error")
	}

	// skip order updates if up-band - down-band < min profit spread
	if (s.boll.LastUpBand() - s.boll.LastDownBand()) <= s.ProfitSpread.Float64() {
		log.Infof("boll: down band price == up band price, skipping...")
		return
	}

	s.placeGridOrders(orderExecutor, session)

	s.activeOrders.Print()
}

func (s *Strategy) submitReverseOrder(order types.Order) {
	var side = order.Side.Reverse()
	var price = order.Price

	switch side {
	case types.SideTypeSell:
		price += s.ProfitSpread.Float64()

	case types.SideTypeBuy:
		price -= s.ProfitSpread.Float64()

	}

	submitOrder := types.SubmitOrder{
		Symbol:      s.Symbol,
		Side:        side,
		Type:        types.OrderTypeLimit,
		Quantity:    order.Quantity,
		Price:       price,
		TimeInForce: "GTC",
	}

	log.Infof("submitting reverse order: %s against %s", submitOrder.String(), order.String())

	createdOrders, err := s.OrderExecutor.SubmitOrders(context.Background(), submitOrder)
	if err != nil {
		log.WithError(err).Errorf("can not place orders")
		return
	}

	s.activeOrders.Add(createdOrders...)
	s.orders.Add(createdOrders...)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if s.GridNum == 0 {
		s.GridNum = 2
	}

	s.boll = s.StandardIndicatorSet.GetBOLL(types.IntervalWindow{
		Interval: s.Interval,
		Window:   21,
	}, 2.0)

	s.orders = bbgo.NewOrderStore()
	s.orders.BindStream(session.Stream)

	// we don't persist orders so that we can not clear the previous orders for now. just need time to support this.
	s.activeOrders = bbgo.NewLocalActiveOrderBook()
	s.activeOrders.BindStream(session.Stream)
	s.activeOrders.OnFilled(func(o types.Order) {
		s.submitReverseOrder(o)
	})

	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		log.Infof("canceling active orders...")

		if err := session.Exchange.CancelOrders(ctx, s.activeOrders.Orders()...); err != nil {
			log.WithError(err).Errorf("cancel order error")
		}
	})

	// avoid using time ticker since we will need back testing here
	session.Stream.OnKLineClosed(func(kline types.KLine) {
		// skip kline events that does not belong to this symbol
		if kline.Symbol != s.Symbol {
			log.Infof("%s != %s", kline.Symbol, s.Symbol)
			return
		}

		if (s.RepostInterval != "" && (s.RepostInterval == kline.Interval)) || s.Interval == kline.Interval {
			// see if we have enough balances and then we create limit orders on the up band and the down band.
			s.updateOrders(orderExecutor, session)
		}
	})

	return nil
}

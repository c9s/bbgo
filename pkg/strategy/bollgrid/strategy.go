package bollgrid

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "bollgrid"

var log = logrus.WithField("strategy", ID)

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
	Quantity fixedpoint.Value `json:"quantity"`

	// activeOrders is the locally maintained active order book of the maker orders.
	activeOrders *bbgo.ActiveOrderBook

	profitOrders *bbgo.ActiveOrderBook

	orders *bbgo.OrderStore

	// boll is the BOLLINGER indicator we used for predicting the price.
	boll *indicator.BOLL

	CancelProfitOrdersOnShutdown bool `json: "shutdownCancelProfitOrders"`
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Validate() error {
	if s.ProfitSpread.Sign() <= 0 {
		// If profitSpread is empty or its value is negative
		return fmt.Errorf("profit spread should bigger than 0")
	}
	if s.Quantity.Sign() <= 0 {
		// If quantity is empty or its value is negative
		return fmt.Errorf("quantity should bigger than 0")
	}
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	if s.Interval == "" {
		panic("bollgrid interval can not be empty")
	}

	// currently we need the 1m kline to update the last close price and indicators
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})

	if len(s.RepostInterval) > 0 && s.Interval != s.RepostInterval {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.RepostInterval})
	}
}

func (s *Strategy) generateGridBuyOrders(session *bbgo.ExchangeSession) ([]types.SubmitOrder, error) {
	balances := session.GetAccount().Balances()
	quoteBalance := balances[s.Market.QuoteCurrency].Available
	if quoteBalance.Sign() <= 0 {
		return nil, fmt.Errorf("quote balance %s is zero: %v", s.Market.QuoteCurrency, quoteBalance)
	}

	upBand, downBand := s.boll.LastUpBand(), s.boll.LastDownBand()
	if upBand <= 0.0 {
		return nil, fmt.Errorf("up band == 0")
	}
	if downBand <= 0.0 {
		return nil, fmt.Errorf("down band == 0")
	}

	currentPrice, ok := session.LastPrice(s.Symbol)
	if !ok {
		return nil, fmt.Errorf("last price not found")
	}

	if currentPrice.Float64() > upBand || currentPrice.Float64() < downBand {
		return nil, fmt.Errorf("current price %v exceed the bollinger band %f <> %f", currentPrice, upBand, downBand)
	}

	ema99 := s.StandardIndicatorSet.EWMA(types.IntervalWindow{Interval: s.Interval, Window: 99})
	ema25 := s.StandardIndicatorSet.EWMA(types.IntervalWindow{Interval: s.Interval, Window: 25})
	ema7 := s.StandardIndicatorSet.EWMA(types.IntervalWindow{Interval: s.Interval, Window: 7})
	if ema7.Last() > ema25.Last()*1.001 && ema25.Last() > ema99.Last()*1.0005 {
		log.Infof("all ema lines trend up, skip buy")
		return nil, nil
	}

	priceRange := upBand - downBand
	gridSize := priceRange / float64(s.GridNum)

	var orders []types.SubmitOrder
	for pricef := upBand; pricef >= downBand; pricef -= gridSize {
		if pricef >= currentPrice.Float64() {
			continue
		}
		price := fixedpoint.NewFromFloat(pricef)
		// adjust buy quantity using current quote balance
		quantity := bbgo.AdjustFloatQuantityByMaxAmount(s.Quantity, price, quoteBalance)
		order := types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        types.SideTypeBuy,
			Type:        types.OrderTypeLimit,
			Market:      s.Market,
			Quantity:    quantity,
			Price:       price,
			TimeInForce: types.TimeInForceGTC,
		}
		quoteQuantity := order.Quantity.Mul(price)
		if quantity.Compare(s.MinQuantity) < 0 {
			// don't submit this order if buy quantity is too small
			log.Infof("quote balance %v is not enough, stop generating buy orders", quoteBalance)
			break
		}
		quoteBalance = quoteBalance.Sub(quoteQuantity)
		log.Infof("submitting order: %s", order.String())
		orders = append(orders, order)
	}
	return orders, nil
}

func (s *Strategy) generateGridSellOrders(session *bbgo.ExchangeSession) ([]types.SubmitOrder, error) {
	balances := session.GetAccount().Balances()
	baseBalance := balances[s.Market.BaseCurrency].Available
	if baseBalance.Sign() <= 0 {
		return nil, fmt.Errorf("base balance %s is zero: %+v", s.Market.BaseCurrency, baseBalance)
	}

	upBand, downBand := s.boll.LastUpBand(), s.boll.LastDownBand()
	if upBand <= 0.0 {
		return nil, fmt.Errorf("up band == 0")
	}
	if downBand <= 0.0 {
		return nil, fmt.Errorf("down band == 0")
	}

	currentPrice, ok := session.LastPrice(s.Symbol)
	if !ok {
		return nil, fmt.Errorf("last price not found")
	}

	currentPricef := currentPrice.Float64()

	if currentPricef > upBand || currentPricef < downBand {
		return nil, fmt.Errorf("current price exceed the bollinger band")
	}

	ema99 := s.StandardIndicatorSet.EWMA(types.IntervalWindow{Interval: s.Interval, Window: 99})
	ema25 := s.StandardIndicatorSet.EWMA(types.IntervalWindow{Interval: s.Interval, Window: 25})
	ema7 := s.StandardIndicatorSet.EWMA(types.IntervalWindow{Interval: s.Interval, Window: 7})
	if ema7.Last() < ema25.Last()*(1-0.004) && ema25.Last() < ema99.Last()*(1-0.0005) {
		log.Infof("all ema lines trend down, skip sell")
		return nil, nil
	}

	priceRange := upBand - downBand
	gridSize := priceRange / float64(s.GridNum)

	var orders []types.SubmitOrder
	for pricef := downBand; pricef <= upBand; pricef += gridSize {
		if pricef <= currentPricef {
			continue
		}
		price := fixedpoint.NewFromFloat(pricef)
		// adjust sell quantity using current base balance
		quantity := fixedpoint.Min(s.Quantity, baseBalance)
		order := types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        types.SideTypeSell,
			Type:        types.OrderTypeLimit,
			Market:      s.Market,
			Quantity:    quantity,
			Price:       price,
			TimeInForce: types.TimeInForceGTC,
		}
		baseQuantity := order.Quantity
		if quantity.Compare(s.MinQuantity) < 0 {
			// don't submit this order if sell quantity is too small
			log.Infof("base balance %s is not enough, stop generating sell orders", baseBalance)
			break
		}
		baseBalance = baseBalance.Sub(baseQuantity)
		log.Infof("submitting order: %s", order.String())
		orders = append(orders, order)
	}
	return orders, nil
}

func (s *Strategy) placeGridOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	sellOrders, err := s.generateGridSellOrders(session)
	if err != nil {
		log.Warn(err.Error())
	}
	createdSellOrders, err := orderExecutor.SubmitOrders(context.Background(), sellOrders...)
	if err != nil {
		log.WithError(err).Errorf("can not place sell orders")
	}

	buyOrders, err := s.generateGridBuyOrders(session)
	if err != nil {
		log.Warn(err.Error())
	}
	createdBuyOrders, err := orderExecutor.SubmitOrders(context.Background(), buyOrders...)
	if err != nil {
		log.WithError(err).Errorf("can not place buy orders")
	}

	createdOrders := append(createdSellOrders, createdBuyOrders...)
	s.activeOrders.Add(createdOrders...)
	s.orders.Add(createdOrders...)
}

func (s *Strategy) updateOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	if err := orderExecutor.CancelOrders(context.Background(), s.activeOrders.Orders()...); err != nil {
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

func (s *Strategy) submitReverseOrder(order types.Order, session *bbgo.ExchangeSession) {
	balances := session.GetAccount().Balances()

	var side = order.Side.Reverse()
	var price = order.Price
	var quantity = order.Quantity

	switch side {
	case types.SideTypeSell:
		price = price.Add(s.ProfitSpread)
		maxQuantity := balances[s.Market.BaseCurrency].Available
		quantity = fixedpoint.Min(quantity, maxQuantity)

	case types.SideTypeBuy:
		price = price.Sub(s.ProfitSpread)
		maxQuantity := balances[s.Market.QuoteCurrency].Available.Div(price)
		quantity = fixedpoint.Min(quantity, maxQuantity)
	}

	submitOrder := types.SubmitOrder{
		Symbol:      s.Symbol,
		Side:        side,
		Type:        types.OrderTypeLimit,
		Quantity:    quantity,
		Price:       price,
		TimeInForce: types.TimeInForceGTC,
	}

	log.Infof("submitting reverse order: %s against %s", submitOrder.String(), order.String())

	createdOrders, err := s.OrderExecutor.SubmitOrders(context.Background(), submitOrder)
	if err != nil {
		log.WithError(err).Errorf("can not place orders")
		return
	}

	s.profitOrders.Add(createdOrders...)
	s.orders.Add(createdOrders...)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if s.GridNum == 0 {
		s.GridNum = 2
	}

	s.boll = s.StandardIndicatorSet.BOLL(types.IntervalWindow{
		Interval: s.Interval,
		Window:   21,
	}, 2.0)

	s.orders = bbgo.NewOrderStore(s.Symbol)
	s.orders.BindStream(session.UserDataStream)

	// we don't persist orders so that we can not clear the previous orders for now. just need time to support this.
	s.activeOrders = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeOrders.OnFilled(func(o types.Order) {
		s.submitReverseOrder(o, session)
	})
	s.activeOrders.BindStream(session.UserDataStream)

	s.profitOrders = bbgo.NewActiveOrderBook(s.Symbol)
	s.profitOrders.OnFilled(func(o types.Order) {
		// we made profit here!
	})
	s.profitOrders.BindStream(session.UserDataStream)

	// setup graceful shutting down handler
	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		// call Done to notify the main process.
		defer wg.Done()
		log.Infof("canceling active orders...")

		if err := orderExecutor.CancelOrders(ctx, s.activeOrders.Orders()...); err != nil {
			log.WithError(err).Errorf("cancel order error")
		}

		if s.CancelProfitOrdersOnShutdown {
			log.Infof("canceling profit orders...")
			err := orderExecutor.CancelOrders(ctx, s.profitOrders.Orders()...)

			if err != nil {
				log.WithError(err).Errorf("cancel profit order error")
			}
		}
	})

	session.UserDataStream.OnStart(func() {
		log.Infof("connected, submitting the first round of the orders")
		s.updateOrders(orderExecutor, session)
	})

	// avoid using time ticker since we will need back testing here
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// skip kline events that does not belong to this symbol
		if kline.Symbol != s.Symbol {
			log.Infof("%s != %s", kline.Symbol, s.Symbol)
			return
		}

		if s.RepostInterval != "" {
			// see if we have enough balances and then we create limit orders on the up band and the down band.
			if s.RepostInterval == kline.Interval {
				s.updateOrders(orderExecutor, session)
			}

		} else if s.Interval == kline.Interval {
			s.updateOrders(orderExecutor, session)
		}
	})

	return nil
}

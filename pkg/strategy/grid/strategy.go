package grid

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

var log = logrus.WithField("strategy", "grid")

// The indicators (SMA and EWMA) that we want to use are returning float64 data.
type Float64Indicator interface {
	Last() float64
}

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy("grid", &Strategy{})
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

	MinProfitSpread fixedpoint.Value `json:"minProfitSpread"`

	// GridNum is the grid number, how many orders you want to post on the orderbook.
	GridNum int `json:"gridNumber"`

	// BaseQuantity is the quantity you want to submit for each order.
	BaseQuantity float64 `json:"baseQuantity"`

	// activeOrders is the locally maintained active order book of the maker orders.
	activeOrders *bbgo.LocalActiveOrderBook

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
	if !ok || balance.Available <= 0.0 {
		return
	}

	var numOrders = s.GridNum - s.activeOrders.NumOfBids()
	if numOrders <= 0 {
		return
	}

	var downBand = s.boll.LastDownBand()
	if downBand <= 0.0 {
		return
	}

	var startPrice = downBand

	var submitOrders []types.SubmitOrder
	for i := 0; i < numOrders; i++ {
		submitOrders = append(submitOrders, types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        types.SideTypeBuy,
			Type:        types.OrderTypeLimit,
			Market:      s.Market,
			Quantity:    s.BaseQuantity,
			Price:       startPrice,
			TimeInForce: "GTC",
		})

		startPrice -= s.GridPips.Float64()
	}

	orders, err := orderExecutor.SubmitOrders(context.Background(), submitOrders...)
	if err != nil {
		log.WithError(err).Error("submit bid order error")
		return
	}

	s.activeOrders.Add(orders...)
}

func (s *Strategy) updateAskOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	baseCurrency := s.Market.BaseCurrency
	balances := session.Account.Balances()

	balance, ok := balances[baseCurrency]
	if !ok || balance.Available <= 0.0 {
		return
	}

	var numOrders = s.GridNum - s.activeOrders.NumOfAsks()
	if numOrders <= 0 {
		return
	}

	var upBand = s.boll.LastUpBand()
	if upBand <= 0.0 {
		return
	}

	var startPrice = upBand

	var submitOrders []types.SubmitOrder
	for i := 0; i < numOrders; i++ {
		submitOrders = append(submitOrders, types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        types.SideTypeSell,
			Type:        types.OrderTypeLimit,
			Market:      s.Market,
			Quantity:    s.BaseQuantity,
			Price:       startPrice,
			TimeInForce: "GTC",
		})

		startPrice += s.GridPips.Float64()
	}

	orders, err := orderExecutor.SubmitOrders(context.Background(), submitOrders...)
	if err != nil {
		log.WithError(err).Error("submit ask order error")
		return
	}

	log.Infof("adding orders to the active ask order pool...")
	s.activeOrders.Add(orders...)
}

func (s *Strategy) updateOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	// skip order updates if up-band - down-band < min profit spread
	if (s.boll.LastUpBand() - s.boll.LastDownBand()) <= s.MinProfitSpread.Float64() {
		log.Infof("boll: down band price == up band price, skipping...")
		return
	}

	if err := session.Exchange.CancelOrders(context.Background(), s.activeOrders.Orders()...); err != nil {
		log.WithError(err).Errorf("cancel order error")
	}

	log.Infof("checking grid orders, bids=%d asks=%d", s.activeOrders.Bids.Len(), s.activeOrders.Asks.Len())
	s.activeOrders.Print()

	if s.activeOrders.Bids.Len() < s.GridNum {
		_, ok := session.Account.Balance(s.Market.QuoteCurrency)
		if ok {
			log.Infof("active bid orders not enough: %d < %d, updating...", s.activeOrders.Bids.Len(), s.GridNum)
			s.updateBidOrders(orderExecutor, session)
		}
	}

	if s.activeOrders.Asks.Len() < s.GridNum {
		_, ok := session.Account.Balance(s.Market.BaseCurrency)

		// TODO: add base asset quantity check, think about how to reuse the risk control executor
		if ok {
			log.Infof("active ask orders not enough: %d < %d, updating...", s.activeOrders.Asks.Len(), s.GridNum)
			s.updateAskOrders(orderExecutor, session)
		}
	}
}

func (s *Strategy) orderUpdateHandler(order types.Order) {
	if order.Symbol != s.Symbol {
		return
	}

	log.Infof("received order update: %+v", order)

	switch order.Status {
	case types.OrderStatusFilled:
		s.WriteOff(order)

	case types.OrderStatusCanceled, types.OrderStatusRejected:
		log.Infof("order status %s, removing %d from the active order pool...", order.Status, order.OrderID)
		s.activeOrders.Delete(order)

	default:
		log.Infof("order status %s, updating %d to the active order pool...", order.Status, order.OrderID)
		s.activeOrders.Add(order)
	}
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if s.GridNum == 0 {
		s.GridNum = 2
	}

	s.boll = s.StandardIndicatorSet.GetBOLL(types.IntervalWindow{
		Interval: s.Interval,
		Window:   21,
	})

	// we don't persist orders so that we can not clear the previous orders for now. just need time to support this.
	s.activeOrders = bbgo.NewLocalActiveOrderBook()

	session.Stream.OnOrderUpdate(s.orderUpdateHandler)

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

	// in order to avoid blocking the stream callbacks, we need to spawn a go routine here to listen to the signal
	go func() {
		for {
			select {
			case <-ctx.Done():
				// TODO: add and fix graceful shutdown
				_ = session.Exchange.CancelOrders(context.Background(), s.activeOrders.Orders()...)
				return
			}
		}
	}()

	return nil
}

// WriteOff writes off the filled order on the opposite side.
// This method does not write off order by order amount or order quantity.
func (s *Strategy) WriteOff(order types.Order) bool {
	b := s.activeOrders
	if order.Status != types.OrderStatusFilled {
		return false
	}

	switch order.Side {
	case types.SideTypeSell:
		// find the filled bid to remove
		if filledOrder, ok := b.Bids.AnyFilled(); ok {
			b.Bids.Delete(filledOrder.OrderID)
			b.Asks.Delete(order.OrderID)
			return true
		}

	case types.SideTypeBuy:
		// find the filled ask order to remove
		if filledOrder, ok := b.Asks.AnyFilled(); ok {
			b.Asks.Delete(filledOrder.OrderID)
			b.Bids.Delete(order.OrderID)
			return true
		}
	}

	return false
}

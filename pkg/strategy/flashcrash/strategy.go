// flashcrash strategy tries to place the orders at 30%~50% of the current price,
// so that you can catch the orders while flashcrash happens
package flashcrash

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "flashcrash"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	// These fields will be filled from the config file (it translates YAML to JSON)
	// Symbol is the symbol of market you want to run this strategy
	Symbol string `json:"symbol"`

	// Interval is the interval used to trigger order updates
	Interval types.Interval `json:"interval"`

	// GridNum is the grid number, how many orders you want to places
	GridNum int `json:"gridNumber"`

	Percentage fixedpoint.Value `json:"percentage"`

	// BaseQuantity is the quantity you want to submit for each order.
	BaseQuantity fixedpoint.Value `json:"baseQuantity"`

	// activeOrders is the locally maintained active order book of the maker orders.
	activeOrders *bbgo.ActiveOrderBook

	// Injection fields start
	// --------------------------
	// Market stores the configuration of the market, for example, VolumePrecision, PricePrecision, MinLotSize... etc
	// This field will be injected automatically since we defined the Symbol field.
	types.Market

	// StandardIndicatorSet contains the standard indicators of a market (symbol)
	// This field will be injected automatically since we defined the Symbol field.
	*bbgo.StandardIndicatorSet

	// ewma is the exponential weighted moving average indicator
	ewma *indicator.EWMA
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) updateOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	if err := s.activeOrders.GracefulCancel(context.Background(), session.Exchange); err != nil {
		log.WithError(err).Errorf("cancel order error")
	}

	s.updateBidOrders(orderExecutor, session)
}

func (s *Strategy) updateBidOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	quoteCurrency := s.Market.QuoteCurrency
	balances := session.GetAccount().Balances()

	balance, ok := balances[quoteCurrency]
	if !ok || balance.Available.Sign() <= 0 {
		log.Infof("insufficient balance of %s: %v", quoteCurrency, balance.Available)
		return
	}

	var startPrice = fixedpoint.NewFromFloat(s.ewma.Last()).Mul(s.Percentage)

	var submitOrders []types.SubmitOrder
	for i := 0; i < s.GridNum; i++ {
		submitOrders = append(submitOrders, types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        types.SideTypeBuy,
			Type:        types.OrderTypeLimit,
			Market:      s.Market,
			Quantity:    s.BaseQuantity,
			Price:       startPrice,
			TimeInForce: types.TimeInForceGTC,
		})

		startPrice = startPrice.Mul(s.Percentage)
	}

	orders, err := orderExecutor.SubmitOrders(context.Background(), submitOrders...)
	if err != nil {
		log.WithError(err).Error("submit bid order error")
		return
	}

	s.activeOrders.Add(orders...)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// we don't persist orders so that we can not clear the previous orders for now. just need time to support this.
	s.activeOrders = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeOrders.BindStream(session.UserDataStream)

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		log.Infof("canceling active orders...")

		if err := orderExecutor.CancelOrders(ctx, s.activeOrders.Orders()...); err != nil {
			log.WithError(err).Errorf("cancel order error")
		}
	})

	s.ewma = s.StandardIndicatorSet.EWMA(types.IntervalWindow{
		Interval: s.Interval,
		Window:   25,
	})

	session.UserDataStream.OnStart(func() {
		s.updateOrders(orderExecutor, session)
	})

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		s.updateOrders(orderExecutor, session)
	})

	return nil
}

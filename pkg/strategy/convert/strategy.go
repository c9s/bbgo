package convert

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util/tradingutil"
)

const ID = "convert"

var log = logrus.WithField("strategy", ID)

var stableCoins = []string{"USDT", "USDC"}

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

// Strategy "convert" converts your specific asset into other asset
type Strategy struct {
	Market types.Market

	From string `json:"from"`
	To   string `json:"to"`

	// Interval is the period that you want to submit order
	Interval types.Interval `json:"interval"`

	UseLimitOrder bool `json:"useLimitOrder"`

	UseTakerOrder bool `json:"useTakerOrder"`

	MinBalance  fixedpoint.Value `json:"minBalance"`
	MaxQuantity fixedpoint.Value `json:"maxQuantity"`

	Position *types.Position `persistence:"position"`

	directMarket    *types.Market
	indirectMarkets []types.Market

	markets       map[string]types.Market
	session       *bbgo.ExchangeSession
	orderExecutor *bbgo.SimpleOrderExecutor

	pendingQuantity     map[string]fixedpoint.Value `persistence:"pendingQuantities"`
	pendingQuantityLock sync.Mutex
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s-%s", ID, s.From, s.To)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {

}

func (s *Strategy) Validate() error {
	return nil
}

func (s *Strategy) handleOrderFilled(ctx context.Context, order types.Order) {
	var fees = map[string]fixedpoint.Value{}

	if service, ok := s.session.Exchange.(types.ExchangeOrderQueryService); ok {
		trades, err := service.QueryOrderTrades(ctx, types.OrderQuery{
			Symbol:  order.Symbol,
			OrderID: strconv.FormatUint(order.OrderID, 10),
		})

		if err != nil {
			return
		}

		fees = tradingutil.CollectTradeFee(trades)
		log.Infof("aggregated order fees: %+v", fees)
	}

	if s.directMarket != nil {
		if order.Symbol != s.directMarket.Symbol {
			return
		}

		// TODO: notification
		return
	} else if len(s.indirectMarkets) > 0 {
		for i := 0; i < len(s.indirectMarkets); i++ {
			market := s.indirectMarkets[i]
			if market.Symbol != order.Symbol {
				continue
			}

			if i == len(s.indirectMarkets)-1 {
				// TODO: handle the final order here
				continue
			}

			nextMarket := s.indirectMarkets[i+1]

			quantity := order.Quantity
			quoteQuantity := quantity.Mul(order.Price)

			switch order.Side {
			case types.SideTypeSell:
				// convert quote asset
				if quoteFee, ok := fees[market.QuoteCurrency]; ok {
					quoteQuantity = quoteQuantity.Sub(quoteFee)
				}

				if err := s.convertBalance(ctx, market.QuoteCurrency, quoteQuantity, nextMarket); err != nil {
					log.WithError(err).Errorf("unable to convert balance")
				}

			case types.SideTypeBuy:
				if baseFee, ok := fees[market.BaseCurrency]; ok {
					quantity = quantity.Sub(baseFee)
				}

				if err := s.convertBalance(ctx, market.BaseCurrency, quantity, nextMarket); err != nil {
					log.WithError(err).Errorf("unable to convert balance")
				}
			}
		}
	}

}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.pendingQuantity = make(map[string]fixedpoint.Value)
	s.session = session
	s.markets = session.Markets()

	if market, ok := findDirectMarket(s.markets, s.From, s.To); ok {
		s.directMarket = &market
	} else if marketChain, ok := findIndirectMarket(s.markets, s.From, s.To); ok {
		s.indirectMarkets = marketChain
	}

	s.orderExecutor = bbgo.NewSimpleOrderExecutor(session)
	s.orderExecutor.ActiveMakerOrders().OnFilled(func(o types.Order) {
		s.handleOrderFilled(ctx, o)
	})
	s.orderExecutor.Bind()

	if s.Interval != "" {
		session.UserDataStream.OnStart(func() {
			go s.tickWatcher(ctx, s.Interval.Duration())
		})
	}

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		s.collectPendingQuantity(ctx)

		_ = s.orderExecutor.GracefulCancel(ctx)
	})

	return nil
}

func (s *Strategy) tickWatcher(ctx context.Context, interval time.Duration) {
	if err := s.convert(ctx); err != nil {
		log.WithError(err).Errorf("unable to convert asset %s", s.From)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			if err := s.convert(ctx); err != nil {
				log.WithError(err).Errorf("unable to convert asset %s", s.From)
			}
		}
	}
}

func (s *Strategy) getSourceMarket() (types.Market, bool) {
	if s.directMarket != nil {
		return *s.directMarket, true
	} else if len(s.indirectMarkets) > 0 {
		return s.indirectMarkets[0], true
	}

	return types.Market{}, false
}

// convert triggers a convert order
func (s *Strategy) convert(ctx context.Context) error {
	s.collectPendingQuantity(ctx)

	// sleep one second for exchange to unlock the balance
	time.Sleep(time.Second)

	account := s.session.GetAccount()
	fromAsset, ok := account.Balance(s.From)
	if !ok {
		return nil
	}

	log.Debugf("converting %s to %s, current balance: %+v", s.From, s.To, fromAsset)

	if sourceMarket, ok := s.getSourceMarket(); ok {
		quantity := fromAsset.Available

		if !s.MinBalance.IsZero() {
			quantity = quantity.Sub(s.MinBalance)
			if quantity.Sign() < 0 {
				return nil
			}
		}

		if !s.MaxQuantity.IsZero() {
			quantity = fixedpoint.Min(s.MaxQuantity, quantity)
		}

		if err := s.convertBalance(ctx, fromAsset.Currency, quantity, sourceMarket); err != nil {
			return err
		}
	}

	return nil
}

func (s *Strategy) addPendingQuantity(asset string, q fixedpoint.Value) {
	if q2, ok := s.pendingQuantity[asset]; ok {
		s.pendingQuantity[asset] = q2.Add(q)
	} else {
		s.pendingQuantity[asset] = q
	}
}

func (s *Strategy) collectPendingQuantity(ctx context.Context) {
	log.Infof("collecting pending quantity...")

	s.pendingQuantityLock.Lock()
	defer s.pendingQuantityLock.Unlock()

	activeOrders := s.orderExecutor.ActiveMakerOrders().Orders()
	log.Infof("found %d active orders", len(activeOrders))

	if err := s.orderExecutor.GracefulCancel(ctx); err != nil {
		log.WithError(err).Warn("unable to cancel orders")
	}

	for _, o := range activeOrders {
		log.Infof("checking order: %+v", o)

		if service, ok := s.session.Exchange.(types.ExchangeOrderQueryService); ok {
			trades, err := service.QueryOrderTrades(ctx, types.OrderQuery{
				Symbol:  o.Symbol,
				OrderID: strconv.FormatUint(o.OrderID, 10),
			})

			if err != nil {
				return
			}

			o.ExecutedQuantity = tradingutil.AggregateTradesQuantity(trades)

			log.Infof("updated executed quantity to %s", o.ExecutedQuantity)
		}

		if m, ok := s.markets[o.Symbol]; ok {
			switch o.Side {
			case types.SideTypeBuy:
				if !o.ExecutedQuantity.IsZero() {
					s.addPendingQuantity(m.BaseCurrency, o.ExecutedQuantity)
				}

				if m.QuoteCurrency == s.From {
					continue
				}

				qq := o.Quantity.Sub(o.ExecutedQuantity).Mul(o.Price)
				s.addPendingQuantity(m.QuoteCurrency, qq)
			case types.SideTypeSell:

				if !o.ExecutedQuantity.IsZero() {
					s.addPendingQuantity(m.QuoteCurrency, o.ExecutedQuantity.Mul(o.Price))
				}

				if m.BaseCurrency == s.From {
					continue
				}

				q := o.Quantity.Sub(o.ExecutedQuantity)
				s.addPendingQuantity(m.BaseCurrency, q)
			}
		}
	}

	log.Infof("collected pending quantity: %+v", s.pendingQuantity)
}

func (s *Strategy) convertBalance(ctx context.Context, fromAsset string, available fixedpoint.Value, market types.Market) error {
	ticker, err2 := s.session.Exchange.QueryTicker(ctx, market.Symbol)
	if err2 != nil {
		return err2
	}

	s.pendingQuantityLock.Lock()
	if pendingQ, ok := s.pendingQuantity[fromAsset]; ok {

		log.Infof("adding pending quantity %s to the current quantity %s", pendingQ, available)
		available = available.Add(pendingQ)

		delete(s.pendingQuantity, fromAsset)
	}
	s.pendingQuantityLock.Unlock()

	switch fromAsset {

	case market.BaseCurrency:
		price := ticker.Sell
		if s.UseTakerOrder {
			price = ticker.Buy
		}

		log.Infof("converting %s %s to %s...", available, fromAsset, market.QuoteCurrency)

		quantity, ok := market.GreaterThanMinimalOrderQuantity(types.SideTypeSell, price, available)
		if !ok {
			log.Debugf("asset %s %s is less than MoQ, skip convert", available, fromAsset)
			return nil
		}

		orderForm := types.SubmitOrder{
			Symbol:      market.Symbol,
			Side:        types.SideTypeSell,
			Type:        types.OrderTypeLimit,
			Quantity:    quantity,
			Price:       price,
			Market:      market,
			TimeInForce: types.TimeInForceGTC,
		}
		if _, err := s.orderExecutor.SubmitOrders(ctx, orderForm); err != nil {
			log.WithError(err).Errorf("unable to submit order: %+v", orderForm)
		}

	case market.QuoteCurrency:
		price := ticker.Buy
		if s.UseTakerOrder {
			price = ticker.Sell
		}

		log.Infof("converting %s %s to %s...", available, fromAsset, market.BaseCurrency)

		quantity, ok := market.GreaterThanMinimalOrderQuantity(types.SideTypeBuy, price, available)
		if !ok {
			log.Debugf("asset %s %s is less than MoQ, skip convert", available, fromAsset)
			return nil
		}

		orderForm := types.SubmitOrder{
			Symbol:      market.Symbol,
			Side:        types.SideTypeBuy,
			Type:        types.OrderTypeLimit,
			Quantity:    quantity,
			Price:       price,
			Market:      market,
			TimeInForce: types.TimeInForceGTC,
		}
		if _, err := s.orderExecutor.SubmitOrders(ctx, orderForm); err != nil {
			log.WithError(err).Errorf("unable to submit order: %+v", orderForm)
		}
	}

	return nil
}

func findIndirectMarket(markets map[string]types.Market, from, to string) ([]types.Market, bool) {
	var sourceMarkets = map[string]types.Market{}
	var targetMarkets = map[string]types.Market{}

	for _, market := range markets {
		if market.BaseCurrency == from {
			sourceMarkets[market.QuoteCurrency] = market
		} else if market.QuoteCurrency == from {
			sourceMarkets[market.BaseCurrency] = market
		}

		if market.BaseCurrency == to {
			targetMarkets[market.QuoteCurrency] = market
		} else if market.QuoteCurrency == to {
			targetMarkets[market.BaseCurrency] = market
		}
	}

	// prefer stable coins for better liquidity
	for _, stableCoin := range stableCoins {
		m1, ok1 := sourceMarkets[stableCoin]
		m2, ok2 := targetMarkets[stableCoin]
		if ok1 && ok2 {
			return []types.Market{m1, m2}, true
		}
	}

	for sourceCurrency, m1 := range sourceMarkets {
		if m2, ok := targetMarkets[sourceCurrency]; ok {
			return []types.Market{m1, m2}, true
		}
	}

	return nil, false
}

func findDirectMarket(markets map[string]types.Market, from, to string) (types.Market, bool) {
	symbol := from + to
	if m, ok := markets[symbol]; ok {
		return m, true
	}

	symbol = to + from
	if m, ok := markets[symbol]; ok {
		return m, true
	}

	return types.Market{}, false
}

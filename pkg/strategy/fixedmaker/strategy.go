package fixedmaker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "fixedmaker"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

// Fixed spread market making strategy
type Strategy struct {
	*common.Strategy

	Environment *bbgo.Environment
	Market      types.Market

	Symbol     string           `json:"symbol"`
	Interval   types.Interval   `json:"interval"`
	Quantity   fixedpoint.Value `json:"quantity"`
	HalfSpread fixedpoint.Value `json:"halfSpread"`
	OrderType  types.OrderType  `json:"orderType"`
	DryRun     bool             `json:"dryRun"`

	activeOrderBook *bbgo.ActiveOrderBook
}

func (s *Strategy) Defaults() error {
	if s.OrderType == "" {
		log.Infof("order type is not set, using limit maker order type")
		s.OrderType = types.OrderTypeLimitMaker
	}
	return nil
}
func (s *Strategy) Initialize() error {
	return nil
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Validate() error {
	if s.Quantity.Float64() <= 0 {
		return fmt.Errorf("quantity should be positive")
	}

	if s.HalfSpread.Float64() <= 0 {
		return fmt.Errorf("halfSpread should be positive")
	}
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.Strategy = &common.Strategy{}
	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, ID, s.InstanceID())

	s.activeOrderBook = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeOrderBook.BindStream(session.UserDataStream)

	s.activeOrderBook.OnFilled(func(order types.Order) {
		if s.activeOrderBook.NumOfOrders() == 0 {
			log.Infof("no active orders, replenish")
			s.replenish(ctx, order.UpdateTime.Time())
		}
	})

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		log.Infof("%s", kline.String())
		s.replenish(ctx, kline.EndTime.Time())
	})

	// the shutdown handler, you can cancel all orders
	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		_ = s.OrderExecutor.GracefulCancel(ctx)
	})
	return nil
}

func (s *Strategy) replenish(ctx context.Context, t time.Time) {
	if err := s.Session.Exchange.CancelOrders(ctx, s.activeOrderBook.Orders()...); err != nil {
		log.WithError(err).Errorf("failed to cancel orders")
	}

	if s.IsHalted(t) {
		log.Infof("circuit break halted, not replenishing")
		return
	}

	submitOrders, err := s.generateSubmitOrders(ctx)
	if err != nil {
		log.WithError(err).Error("failed to generate submit orders")
		return
	}
	log.Infof("submit orders: %+v", submitOrders)

	if s.DryRun {
		log.Infof("dry run, not submitting orders")
		return
	}

	createdOrders, err := s.OrderExecutor.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		log.WithError(err).Error("failed to submit orders")
		return
	}
	log.Infof("created orders: %+v", createdOrders)

	s.activeOrderBook.Add(createdOrders...)
}

func (s *Strategy) generateSubmitOrders(ctx context.Context) ([]types.SubmitOrder, error) {
	orders := []types.SubmitOrder{}

	baseBalance, ok := s.Session.GetAccount().Balance(s.Market.BaseCurrency)
	if !ok {
		return nil, fmt.Errorf("base currency %s balance not found", s.Market.BaseCurrency)
	}
	log.Infof("base balance: %+v", baseBalance)

	quoteBalance, ok := s.Session.GetAccount().Balance(s.Market.QuoteCurrency)
	if !ok {
		return nil, fmt.Errorf("quote currency %s balance not found", s.Market.QuoteCurrency)
	}
	log.Infof("quote balance: %+v", quoteBalance)

	ticker, err := s.Session.Exchange.QueryTicker(ctx, s.Symbol)
	if err != nil {
		return nil, err
	}
	midPrice := ticker.Buy.Add(ticker.Sell).Div(fixedpoint.NewFromFloat(2.0))
	log.Infof("mid price: %+v", midPrice)

	// calculate bid and ask price
	// sell price = mid price * (1 + r))
	// buy price = mid price * (1 - r))
	sellPrice := midPrice.Mul(fixedpoint.One.Add(s.HalfSpread)).Round(s.Market.PricePrecision, fixedpoint.Up)
	buyPrice := midPrice.Mul(fixedpoint.One.Sub(s.HalfSpread)).Round(s.Market.PricePrecision, fixedpoint.Down)
	log.Infof("sell price: %s, buy price: %s", sellPrice.String(), buyPrice.String())

	// check balance and generate orders
	amount := s.Quantity.Mul(buyPrice)
	if quoteBalance.Available.Compare(amount) > 0 {
		orders = append(orders, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeBuy,
			Type:     s.OrderType,
			Price:    buyPrice,
			Quantity: s.Quantity,
		})
	} else {
		log.Infof("not enough quote balance to buy, available: %s, amount: %s", quoteBalance.Available, amount)
	}

	if baseBalance.Available.Compare(s.Quantity) > 0 {
		orders = append(orders, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeSell,
			Type:     s.OrderType,
			Price:    sellPrice,
			Quantity: s.Quantity,
		})
	} else {
		log.Infof("not enough base balance to sell, available: %s, quantity: %s", baseBalance.Available, s.Quantity)
	}

	return orders, nil
}

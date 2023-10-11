package randomtrader

import (
	"context"
	"fmt"
	"math/rand"
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "randomtrader"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	*common.Strategy

	Environment *bbgo.Environment
	Market      types.Market

	Symbol         string           `json:"symbol"`
	CronExpression string           `json:"cronExpression"`
	Quantity       fixedpoint.Value `json:"quantity"`
	AdjustQuantity bool             `json:"adjustQuantity"`
	OnStart        bool             `json:"onStart"`
	DryRun         bool             `json:"dryRun"`

	cron *cron.Cron
}

func (s *Strategy) Defaults() error {
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
	if s.CronExpression == "" {
		return fmt.Errorf("cronExpression is required")
	}
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.Strategy = &common.Strategy{}
	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, s.ID(), s.InstanceID())

	session.UserDataStream.OnStart(func() {
		if s.OnStart {
			s.placeOrder()
		}
	})

	// the shutdown handler, you can cancel all orders
	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		_ = s.OrderExecutor.GracefulCancel(ctx)
	})

	s.cron = cron.New()
	s.cron.AddFunc(s.CronExpression, s.placeOrder)
	s.cron.Start()

	return nil
}

func (s *Strategy) placeOrder() {
	ctx := context.Background()

	baseBalance, ok := s.Session.GetAccount().Balance(s.Market.BaseCurrency)
	if !ok {
		log.Errorf("base balance not found")
		return
	}
	quoteBalance, ok := s.Session.GetAccount().Balance(s.Market.QuoteCurrency)
	if !ok {
		log.Errorf("quote balance not found")
		return
	}

	ticker, err := s.Session.Exchange.QueryTicker(ctx, s.Symbol)
	if err != nil {
		log.WithError(err).Error("query ticker error")
		return
	}

	sellQuantity := s.Quantity
	buyQuantity := s.Quantity
	if s.AdjustQuantity {
		sellQuantity = s.Market.AdjustQuantityByMinNotional(s.Quantity, ticker.Sell)
		buyQuantity = fixedpoint.Max(s.Quantity, s.Market.MinQuantity)
	}

	orderForm := []types.SubmitOrder{}
	if baseBalance.Available.Compare(sellQuantity) > 0 {
		orderForm = append(orderForm, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeMarket,
			Quantity: sellQuantity,
		})
	} else {
		log.Infof("base balance: %s is not enough", baseBalance.Available.String())
	}

	if quoteBalance.Available.Div(ticker.Buy).Compare(buyQuantity) > 0 {
		orderForm = append(orderForm, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeMarket,
			Quantity: buyQuantity,
		})
	} else {
		log.Infof("quote balance: %s is not enough", quoteBalance.Available.String())
	}

	var order types.SubmitOrder
	if len(orderForm) == 0 {
		log.Infof("both base and quote balance are not enough, skip submit order")
		return
	} else {
		order = orderForm[rand.Intn(len(orderForm))]
	}
	log.Infof("submit order: %s", order.String())

	if s.DryRun {
		log.Infof("dry run, skip submit order")
		return
	}

	_, err = s.OrderExecutor.SubmitOrders(ctx, order)
	if err != nil {
		log.WithError(err).Error("submit order error")
		return
	}
}

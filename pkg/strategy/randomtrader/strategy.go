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
	OnStart        bool             `json:"onStart"`
	DryRun         bool             `json:"dryRun"`
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
			s.trade(ctx)
		}
	})

	// the shutdown handler, you can cancel all orders
	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		_ = s.OrderExecutor.GracefulCancel(ctx)
	})

	cron := cron.New()
	cron.AddFunc(s.CronExpression, func() {
		s.trade(ctx)
	})
	cron.Start()

	return nil
}

func (s *Strategy) trade(ctx context.Context) {
	orderForm := []types.SubmitOrder{
		{
			Symbol:   s.Symbol,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeMarket,
			Quantity: s.Quantity,
		}, {
			Symbol:   s.Symbol,
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeMarket,
			Quantity: s.Quantity,
		},
	}

	submitOrder := orderForm[rand.Intn(2)]
	log.Infof("submit order: %s", submitOrder.String())

	if s.DryRun {
		log.Infof("dry run, skip submit order")
		return
	}

	_, err := s.OrderExecutor.SubmitOrders(ctx, submitOrder)
	if err != nil {
		log.WithError(err).Error("submit order error")
		return
	}
}

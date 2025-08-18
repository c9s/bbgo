package random

import (
	"context"
	"fmt"
	"math/rand"
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "random"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	*common.Strategy
	*common.FeeBudget

	Environment *bbgo.Environment
	Market      types.Market

	Symbol   string `json:"symbol"`
	Schedule string `json:"schedule"`
	OnStart  bool   `json:"onStart"`
	DryRun   bool   `json:"dryRun"`

	bbgo.QuantityOrAmount
	cron *cron.Cron
}

func (s *Strategy) Defaults() error {
	return nil
}

func (s *Strategy) Initialize() error {
	if s.Strategy == nil {
		s.Strategy = &common.Strategy{}
	}

	if s.FeeBudget == nil {
		s.FeeBudget = &common.FeeBudget{}
	}
	return nil
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Validate() error {
	if s.Schedule == "" {
		return fmt.Errorf("schedule is required")
	}

	if err := s.QuantityOrAmount.Validate(); err != nil {
		return err
	}
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, s.ID(), s.InstanceID())
	s.FeeBudget.Initialize()

	session.UserDataStream.OnStart(func() {
		if !s.OnStart {
			return
		}

		if !s.FeeBudget.IsBudgetAllowed() {
			return
		}

		s.placeOrder(ctx)
	})

	session.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
		if trade.Symbol != s.Symbol {
			return
		}
		s.FeeBudget.HandleTradeUpdate(trade)
	})

	// the shutdown handler, you can cancel all orders
	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		_ = s.OrderExecutor.GracefulCancel(ctx)
		bbgo.Sync(ctx, s)
	})

	s.cron = cron.New()
	s.cron.AddFunc(s.Schedule, func() {
		if !s.FeeBudget.IsBudgetAllowed() {
			return
		}

		s.placeOrder(ctx)
	})
	s.cron.Start()

	return nil
}

func (s *Strategy) placeOrder(ctx context.Context) {
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

	sellQuantity := s.CalculateQuantity(ticker.Sell)
	buyQuantity := s.CalculateQuantity(ticker.Buy)
	sellQuantity = s.Market.AdjustQuantityByMinNotional(sellQuantity, ticker.Sell)
	buyQuantity = s.Market.AdjustQuantityByMinNotional(buyQuantity, ticker.Buy)

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

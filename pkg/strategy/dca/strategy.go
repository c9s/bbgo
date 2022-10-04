package dca

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "dca"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type BudgetPeriod string

const (
	BudgetPeriodDay   BudgetPeriod = "day"
	BudgetPeriodWeek  BudgetPeriod = "week"
	BudgetPeriodMonth BudgetPeriod = "month"
)

func (b BudgetPeriod) Duration() time.Duration {
	var period time.Duration
	switch b {
	case BudgetPeriodDay:
		period = 24 * time.Hour

	case BudgetPeriodWeek:
		period = 24 * time.Hour * 7

	case BudgetPeriodMonth:
		period = 24 * time.Hour * 30

	}

	return period
}

// Strategy is the Dollar-Cost-Average strategy
type Strategy struct {
	Environment *bbgo.Environment
	Symbol      string `json:"symbol"`
	Market      types.Market

	// BudgetPeriod is how long your budget quota will be reset.
	// day, week, month
	BudgetPeriod BudgetPeriod `json:"budgetPeriod"`

	// Budget is the amount you invest per budget period
	Budget fixedpoint.Value `json:"budget"`

	// InvestmentInterval is the interval of each investment
	InvestmentInterval types.Interval `json:"investmentInterval"`

	budgetPerInvestment fixedpoint.Value

	Position              *types.Position    `persistence:"position"`
	ProfitStats           *types.ProfitStats `persistence:"profit_stats"`
	BudgetQuota           fixedpoint.Value   `persistence:"budget_quota"`
	BudgetPeriodStartTime time.Time          `persistence:"budget_period_start_time"`

	session       *bbgo.ExchangeSession
	orderExecutor *bbgo.GeneralOrderExecutor

	bbgo.StrategyController
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.InvestmentInterval})
}

func (s *Strategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	return s.orderExecutor.ClosePosition(ctx, percentage)
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if s.BudgetQuota.IsZero() {
		s.BudgetQuota = s.Budget
	}

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	instanceID := s.InstanceID()
	s.session = session
	s.orderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.orderExecutor.BindEnvironment(s.Environment)
	s.orderExecutor.BindProfitStats(s.ProfitStats)
	s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})
	s.orderExecutor.Bind()

	numOfInvestmentPerPeriod := fixedpoint.NewFromFloat(float64(s.BudgetPeriod.Duration()) / float64(s.InvestmentInterval.Duration()))
	s.budgetPerInvestment = s.Budget.Div(numOfInvestmentPerPeriod)

	session.UserDataStream.OnStart(func() {})
	session.MarketDataStream.OnKLine(func(kline types.KLine) {})
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != s.Symbol || kline.Interval != s.InvestmentInterval {
			return
		}

		if s.BudgetPeriodStartTime == (time.Time{}) {
			s.BudgetPeriodStartTime = kline.StartTime.Time().Truncate(time.Minute)
		}

		if kline.EndTime.Time().Sub(s.BudgetPeriodStartTime) >= s.BudgetPeriod.Duration() {
			// reset budget quota
			s.BudgetQuota = s.Budget
			s.BudgetPeriodStartTime = kline.StartTime.Time()
		}

		// check if we have quota
		if s.BudgetQuota.Compare(s.budgetPerInvestment) <= 0 {
			return
		}

		price := kline.Close
		quantity := s.budgetPerInvestment.Div(price)

		_, err := s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeMarket,
			Quantity: quantity,
			Market:   s.Market,
		})
		if err != nil {
			log.WithError(err).Errorf("submit order failed")
		}
	})

	return nil
}

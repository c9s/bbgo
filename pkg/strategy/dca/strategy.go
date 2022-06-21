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
	*bbgo.Graceful
	*bbgo.Persistence

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

	activeMakerOrders *bbgo.ActiveOrderBook
	orderStore        *bbgo.OrderStore
	tradeCollector    *bbgo.TradeCollector

	session *bbgo.ExchangeSession

	bbgo.StrategyController
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.InvestmentInterval})
}

func (s *Strategy) submitOrders(ctx context.Context, orderExecutor bbgo.OrderExecutor, submitOrders ...types.SubmitOrder) {
	createdOrders, err := orderExecutor.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		log.WithError(err).Errorf("can not place orders")
	}

	s.orderStore.Add(createdOrders...)
	s.activeMakerOrders.Add(createdOrders...)
	s.tradeCollector.Process()
}

func (s *Strategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	base := s.Position.GetBase()
	if base.IsZero() {
		return fmt.Errorf("no opened %s position", s.Position.Symbol)
	}

	// make it negative
	quantity := base.Mul(percentage).Abs()
	side := types.SideTypeBuy
	if base.Sign() > 0 {
		side = types.SideTypeSell
	}

	if quantity.Compare(s.Market.MinQuantity) < 0 {
		return fmt.Errorf("order quantity %v is too small, less than %v", quantity, s.Market.MinQuantity)
	}

	submitOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     side,
		Type:     types.OrderTypeMarket,
		Quantity: quantity,
		Market:   s.Market,
	}

	// s.Notify("Submitting %s %s order to close position by %v", s.Symbol, side.String(), percentage, submitOrder)

	createdOrders, err := s.session.Exchange.SubmitOrders(ctx, submitOrder)
	if err != nil {
		log.WithError(err).Errorf("can not place position close order")
	}

	s.orderStore.Add(createdOrders...)
	s.activeMakerOrders.Add(createdOrders...)
	s.tradeCollector.Process()
	return err
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

// check if position can be close or not
func canClosePosition(position *types.Position, signal fixedpoint.Value, price fixedpoint.Value) bool {
	return !signal.IsZero() && position.IsShort() && !position.IsDust(price)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// initial required information
	s.session = session

	s.activeMakerOrders = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeMakerOrders.BindStream(session.UserDataStream)

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.UserDataStream)

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	instanceID := s.InstanceID()

	if s.BudgetQuota.IsZero() {
		s.BudgetQuota = s.Budget
	}

	numOfInvestmentPerPeriod := fixedpoint.NewFromFloat(float64(s.BudgetPeriod.Duration()) / float64(s.InvestmentInterval.Duration()))
	s.budgetPerInvestment = s.Budget.Div(numOfInvestmentPerPeriod)

	// Always update the position fields
	s.Position.Strategy = ID
	s.Position.StrategyInstanceID = instanceID

	s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, s.Position, s.orderStore)
	s.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
		bbgo.Notify(trade)
		s.ProfitStats.AddTrade(trade)

		if profit.Compare(fixedpoint.Zero) == 0 {
			s.Environment.RecordPosition(s.Position, trade, nil)
		} else {
			log.Infof("%s generated profit: %v", s.Symbol, profit)
			p := s.Position.NewProfit(trade, profit, netProfit)
			p.Strategy = ID
			p.StrategyInstanceID = instanceID
			bbgo.Notify(&p)

			s.ProfitStats.AddProfit(p)
			bbgo.Notify(&s.ProfitStats)

			s.Environment.RecordPosition(s.Position, trade, &p)
		}
	})

	s.tradeCollector.OnTrade(func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {
		s.BudgetQuota = s.BudgetQuota.Sub(trade.QuoteQuantity)
	})

	s.tradeCollector.OnPositionUpdate(func(position *types.Position) {
		log.Infof("position changed: %s", s.Position)
		bbgo.Notify(s.Position)
	})

	s.tradeCollector.BindStream(session.UserDataStream)

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

		s.submitOrders(ctx, orderExecutor, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeMarket,
			Quantity: quantity,
			Market:   s.Market,
		})
	})

	return nil
}

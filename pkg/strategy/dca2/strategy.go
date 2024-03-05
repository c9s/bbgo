package dca2

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/c9s/bbgo/pkg/util/tradingutil"
)

const (
	ID       = "dca2"
	orderTag = "dca2"
)

var (
	log        = logrus.WithField("strategy", ID)
	baseLabels prometheus.Labels
)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type advancedOrderCancelApi interface {
	CancelAllOrders(ctx context.Context) ([]types.Order, error)
	CancelOrdersBySymbol(ctx context.Context, symbol string) ([]types.Order, error)
	CancelOrdersByGroupID(ctx context.Context, groupID uint32) ([]types.Order, error)
}

//go:generate callbackgen -type Strateg
type Strategy struct {
	Position    *types.Position `json:"position,omitempty" persistence:"position"`
	ProfitStats *ProfitStats    `json:"profitStats,omitempty" persistence:"profit_stats"`

	Environment     *bbgo.Environment
	ExchangeSession *bbgo.ExchangeSession
	OrderExecutor   *bbgo.GeneralOrderExecutor
	Market          types.Market

	Symbol string `json:"symbol"`

	// setting
	QuoteInvestment  fixedpoint.Value `json:"quoteInvestment"`
	MaxOrderCount    int64            `json:"maxOrderCount"`
	PriceDeviation   fixedpoint.Value `json:"priceDeviation"`
	TakeProfitRatio  fixedpoint.Value `json:"takeProfitRatio"`
	CoolDownInterval types.Duration   `json:"coolDownInterval"`

	// OrderGroupID is the group ID used for the strategy instance for canceling orders
	OrderGroupID uint32 `json:"orderGroupID"`

	// RecoverWhenStart option is used for recovering dca states
	RecoverWhenStart bool `json:"recoverWhenStart"`

	// KeepOrdersWhenShutdown option is used for keeping the grid orders when shutting down bbgo
	KeepOrdersWhenShutdown bool `json:"keepOrdersWhenShutdown"`

	// UseCancelAllOrdersApiWhenClose uses a different API to cancel all the orders on the market when closing a grid
	UseCancelAllOrdersApiWhenClose bool `json:"useCancelAllOrdersApiWhenClose"`

	// dev mode
	DevMode *DevMode `json:"devMode"`

	// log
	logger    *logrus.Entry
	LogFields logrus.Fields `json:"logFields"`

	// PrometheusLabels will be used as the base prometheus labels
	PrometheusLabels prometheus.Labels `json:"prometheusLabels"`

	// private field
	mu                   sync.Mutex
	takeProfitPrice      fixedpoint.Value
	startTimeOfNextRound time.Time
	nextStateC           chan State
	state                State

	// callbacks
	common.StatusCallbacks
	positionCallbacks []func(*types.Position)
	profitCallbacks   []func(*ProfitStats)
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Validate() error {
	if s.MaxOrderCount < 1 {
		return fmt.Errorf("maxOrderCount can not be < 1")
	}

	if s.TakeProfitRatio.Sign() <= 0 {
		return fmt.Errorf("takeProfitSpread can not be <= 0")
	}

	if s.PriceDeviation.Sign() <= 0 {
		return fmt.Errorf("margin can not be <= 0")
	}

	// TODO: validate balance is enough
	return nil
}

func (s *Strategy) Defaults() error {
	if s.LogFields == nil {
		s.LogFields = logrus.Fields{}
	}

	s.LogFields["symbol"] = s.Symbol
	s.LogFields["strategy"] = ID

	return nil
}

func (s *Strategy) Initialize() error {
	s.logger = log.WithFields(s.LogFields)
	return nil
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s-%s", ID, s.Symbol)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})
}

func (s *Strategy) newPrometheusLabels() prometheus.Labels {
	labels := prometheus.Labels{
		"exchange": "default",
		"symbol":   s.Symbol,
	}

	if s.ExchangeSession != nil {
		labels["exchange"] = s.ExchangeSession.Name
	}

	if s.PrometheusLabels == nil {
		return labels
	}

	return mergeLabels(s.PrometheusLabels, labels)
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	instanceID := s.InstanceID()
	s.ExchangeSession = session
	if s.ProfitStats == nil {
		s.ProfitStats = newProfitStats(s.Market, s.QuoteInvestment)
	}

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	// prometheus
	if s.PrometheusLabels != nil {
		initMetrics(labelKeys(s.PrometheusLabels))
	}
	registerMetrics()

	// prometheus labels
	baseLabels = s.newPrometheusLabels()

	// if dev mode is on and it's not a new strategy
	if s.DevMode != nil && s.DevMode.Enabled && !s.DevMode.IsNewAccount {
		s.ProfitStats = newProfitStats(s.Market, s.QuoteInvestment)
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	s.Position.Strategy = ID
	s.Position.StrategyInstanceID = instanceID

	if session.MakerFeeRate.Sign() > 0 || session.TakerFeeRate.Sign() > 0 {
		s.Position.SetExchangeFeeRate(session.ExchangeName, types.ExchangeFee{
			MakerFeeRate: session.MakerFeeRate,
			TakerFeeRate: session.TakerFeeRate,
		})
	}

	s.OrderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.OrderExecutor.BindEnvironment(s.Environment)
	s.OrderExecutor.Bind()

	if s.OrderGroupID == 0 {
		s.OrderGroupID = util.FNV32(instanceID) % math.MaxInt32
	}

	// order executor
	s.OrderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		s.logger.Infof("[DCA] POSITION UPDATE: %s", s.Position.String())
		bbgo.Sync(ctx, s)

		// update take profit price here
		s.updateTakeProfitPrice()
	})

	s.OrderExecutor.ActiveMakerOrders().OnFilled(func(o types.Order) {
		s.logger.Infof("[DCA] FILLED ORDER: %s", o.String())
		openPositionSide := types.SideTypeBuy
		takeProfitSide := types.SideTypeSell

		switch o.Side {
		case openPositionSide:
			s.emitNextState(OpenPositionOrderFilled)
		case takeProfitSide:
			s.emitNextState(WaitToOpenPosition)
		default:
			s.logger.Infof("[DCA] unsupported side (%s) of order: %s", o.Side, o)
		}

		// update metrics when filled
		s.updateNumOfOrdersMetrics(ctx)
	})

	session.MarketDataStream.OnKLine(func(kline types.KLine) {
		// check price here
		if s.state != OpenPositionOrderFilled {
			return
		}

		compRes := kline.Close.Compare(s.takeProfitPrice)
		// price doesn't hit the take profit price
		if compRes < 0 {
			return
		}

		s.emitNextState(OpenPositionOrdersCancelling)
	})

	session.UserDataStream.OnAuth(func() {
		s.logger.Info("[DCA] user data stream authenticated")
		time.AfterFunc(3*time.Second, func() {
			if isInitialize := s.initializeNextStateC(); !isInitialize {

				// no need to recover when two situation
				// 1. recoverWhenStart is false
				// 2. dev mode is on and it's not new strategy
				if !s.RecoverWhenStart || (s.DevMode != nil && s.DevMode.Enabled && !s.DevMode.IsNewAccount) {
					s.updateState(WaitToOpenPosition)
				} else {
					// recover
					if err := s.recover(ctx); err != nil {
						s.logger.WithError(err).Error("[DCA] something wrong when state recovering")
						return
					}
				}

				s.logger.Infof("[DCA] state: %d", s.state)
				s.logger.Infof("[DCA] position %s", s.Position.String())
				s.logger.Infof("[DCA] profit stats %s", s.ProfitStats.String())
				s.logger.Infof("[DCA] startTimeOfNextRound %s", s.startTimeOfNextRound)

				s.updateTakeProfitPrice()

				// store persistence
				bbgo.Sync(ctx, s)

				// ready
				s.EmitReady()

				// start running state machine
				s.runState(ctx)
			}
		})
	})

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		if s.KeepOrdersWhenShutdown {
			s.logger.Infof("keepOrdersWhenShutdown is set, will keep the orders on the exchange")
			return
		}

		if err := s.Close(ctx); err != nil {
			s.logger.WithError(err).Errorf("dca2 graceful order cancel error")
		}
	})

	return nil
}

func (s *Strategy) updateTakeProfitPrice() {
	takeProfitRatio := s.TakeProfitRatio
	s.takeProfitPrice = s.Market.TruncatePrice(s.Position.AverageCost.Mul(fixedpoint.One.Add(takeProfitRatio)))
	s.logger.Infof("[DCA] cost: %s, ratio: %s, price: %s", s.Position.AverageCost, takeProfitRatio, s.takeProfitPrice)
}

func (s *Strategy) Close(ctx context.Context) error {
	s.logger.Infof("[DCA] closing %s dca2", s.Symbol)

	defer s.EmitClosed()

	err := s.OrderExecutor.GracefulCancel(ctx)
	if err != nil {
		s.logger.WithError(err).Errorf("[DCA] there are errors when cancelling orders at close")
	}

	bbgo.Sync(ctx, s)
	return err
}

func (s *Strategy) CleanUp(ctx context.Context) error {
	_ = s.Initialize()
	defer s.EmitClosed()

	session := s.ExchangeSession
	if session == nil {
		return fmt.Errorf("Session is nil, please check it")
	}

	// ignore the first cancel error, this skips one open-orders query request
	if err := tradingutil.UniversalCancelAllOrders(ctx, session.Exchange, nil); err == nil {
		return nil
	}

	// if cancel all orders returns error, get the open orders and retry the cancel in each round
	var werr error
	for {
		s.logger.Infof("checking %s open orders...", s.Symbol)

		openOrders, err := retry.QueryOpenOrdersUntilSuccessful(ctx, session.Exchange, s.Symbol)
		if err != nil {
			s.logger.WithError(err).Errorf("unable to query open orders")
			continue
		}

		// all clean up
		if len(openOrders) == 0 {
			break
		}

		if err := tradingutil.UniversalCancelAllOrders(ctx, session.Exchange, openOrders); err != nil {
			s.logger.WithError(err).Errorf("unable to cancel all orders")
			werr = multierr.Append(werr, err)
		}

		time.Sleep(1 * time.Second)
	}

	return werr
}

func (s *Strategy) CalculateAndEmitProfit(ctx context.Context) error {
	historyService, ok := s.ExchangeSession.Exchange.(types.ExchangeTradeHistoryService)
	if !ok {
		return fmt.Errorf("exchange %s doesn't support ExchangeTradeHistoryService", s.ExchangeSession.Exchange.Name())
	}

	queryService, ok := s.ExchangeSession.Exchange.(types.ExchangeOrderQueryService)
	if !ok {
		return fmt.Errorf("exchange %s doesn't support ExchangeOrderQueryService", s.ExchangeSession.Exchange.Name())
	}

	// TODO: pagination for it
	// query the orders
	orders, err := historyService.QueryClosedOrders(ctx, s.Symbol, time.Time{}, time.Time{}, s.ProfitStats.FromOrderID)
	if err != nil {
		return err
	}

	var rounds []Round
	var round Round
	for _, order := range orders {
		// skip not this strategy order
		if order.GroupID != s.OrderGroupID {
			continue
		}

		switch order.Side {
		case types.SideTypeBuy:
			round.OpenPositionOrders = append(round.OpenPositionOrders, order)
		case types.SideTypeSell:
			if order.Status != types.OrderStatusFilled {
				continue
			}
			round.TakeProfitOrder = order
			rounds = append(rounds, round)
			round = Round{}
		default:
			s.logger.Errorf("there is order with unsupported side")
		}
	}

	for _, round := range rounds {
		var roundOrders []types.Order = round.OpenPositionOrders
		roundOrders = append(roundOrders, round.TakeProfitOrder)
		for _, order := range roundOrders {
			s.logger.Infof("[DCA] calculate profit stats from order: %s", order.String())

			// skip no trade orders
			if order.ExecutedQuantity.Sign() == 0 {
				continue
			}

			trades, err := queryService.QueryOrderTrades(ctx, types.OrderQuery{
				Symbol:  order.Symbol,
				OrderID: strconv.FormatUint(order.OrderID, 10),
			})

			if err != nil {
				return err
			}

			for _, trade := range trades {
				s.logger.Infof("[DCA] calculate profit stats from trade: %s", trade.String())
				s.ProfitStats.AddTrade(trade)
			}
		}

		s.ProfitStats.FromOrderID = round.TakeProfitOrder.OrderID + 1
		s.ProfitStats.QuoteInvestment = s.ProfitStats.QuoteInvestment.Add(s.ProfitStats.CurrentRoundProfit)

		// store into persistence
		bbgo.Sync(ctx, s)

		s.logger.Infof("[DCA] profit stats:\n%s", s.ProfitStats.String())

		// emit profit
		s.EmitProfit(s.ProfitStats)
		updateProfitMetrics(s.ProfitStats.Round, s.ProfitStats.CurrentRoundProfit.Float64())

		s.ProfitStats.NewRound()
	}

	return nil
}

func (s *Strategy) updateNumOfOrdersMetrics(ctx context.Context) {
	// update open orders metrics
	openOrders, err := s.ExchangeSession.Exchange.QueryOpenOrders(ctx, s.Symbol)
	if err != nil {
		s.logger.WithError(err).Warn("failed to query open orders to update num of the orders metrics")
	} else {
		metricsNumOfOpenOrders.With(baseLabels).Set(float64(len(openOrders)))
	}

	// update active orders metrics
	metricsNumOfActiveOrders.With(baseLabels).Set(float64(s.OrderExecutor.ActiveMakerOrders().NumOfOrders()))
}

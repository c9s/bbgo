package dca2

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
)

const ID = "dca2"

const orderTag = "dca2"

var log = logrus.WithField("strategy", ID)

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

	Environment   *bbgo.Environment
	Session       *bbgo.ExchangeSession
	OrderExecutor *bbgo.GeneralOrderExecutor
	Market        types.Market

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

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	instanceID := s.InstanceID()
	s.Session = session
	if s.ProfitStats == nil {
		s.ProfitStats = newProfitStats(s.Market, s.QuoteInvestment)
	}

	if s.Position == nil {
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
				if s.RecoverWhenStart {
					// recover
					if err := s.recover(ctx); err != nil {
						s.logger.WithError(err).Error("[DCA] something wrong when state recovering")
						return
					}
				} else {
					s.state = WaitToOpenPosition
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

	session := s.Session
	if session == nil {
		return fmt.Errorf("Session is nil, please check it")
	}

	service, support := session.Exchange.(advancedOrderCancelApi)
	if !support {
		return fmt.Errorf("advancedOrderCancelApi interface is not implemented, fallback to default graceful cancel, exchange %T", session)
	}

	var werr error
	for {
		s.logger.Infof("checking %s open orders...", s.Symbol)

		openOrders, err := retry.QueryOpenOrdersUntilSuccessful(ctx, session.Exchange, s.Symbol)
		if err != nil {
			s.logger.WithError(err).Errorf("CancelOrdersByGroupID api call error")
			werr = multierr.Append(werr, err)
		}

		if len(openOrders) == 0 {
			break
		}

		s.logger.Infof("found %d open orders left, using cancel all orders api", len(openOrders))

		s.logger.Infof("using cancal all orders api for canceling grid orders...")
		if err := retry.CancelAllOrdersUntilSuccessful(ctx, service); err != nil {
			s.logger.WithError(err).Errorf("CancelAllOrders api call error")
			werr = multierr.Append(werr, err)
		}

		time.Sleep(1 * time.Second)
	}

	return werr
}

func (s *Strategy) CalculateAndEmitProfit(ctx context.Context) error {
	historyService, ok := s.Session.Exchange.(types.ExchangeTradeHistoryService)
	if !ok {
		return fmt.Errorf("exchange %s doesn't support ExchangeTradeHistoryService", s.Session.Exchange.Name())
	}

	queryService, ok := s.Session.Exchange.(types.ExchangeOrderQueryService)
	if !ok {
		return fmt.Errorf("exchange %s doesn't support ExchangeOrderQueryService", s.Session.Exchange.Name())
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

		// emit profit
		s.EmitProfit(s.ProfitStats)

		s.ProfitStats.NewRound()
	}

	return nil
}

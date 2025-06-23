package dca2

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/strategy/dca2/statemachine"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/c9s/bbgo/pkg/util/tradingutil"
)

const (
	ID       = "dca2"
	orderTag = "dca2"

	OpenPositionSide = types.SideTypeBuy
	TakeProfitSide   = types.SideTypeSell
)

var (
	recoverSinceLimit = time.Date(2024, time.January, 29, 12, 0, 0, 0, time.Local)
	log               = logrus.WithField("strategy", ID)
	baseLabels        prometheus.Labels
)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

//go:generate callbackgen -type Strateg
type Strategy struct {
	Position       *types.Position `json:"position,omitempty" persistence:"position"`
	ProfitStats    *ProfitStats    `json:"profitStats,omitempty" persistence:"profit_stats"`
	PersistenceTTL types.Duration  `json:"persistenceTTL"`

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
	OrderGroupID              uint32 `json:"orderGroupID"`
	DisableOrderGroupIDFilter bool   `json:"disableOrderGroupIDFilter"`

	// RecoverWhenStart option is used for recovering dca states
	RecoverWhenStart          bool `json:"recoverWhenStart"`
	DisableProfitStatsRecover bool `json:"disableProfitStatsRecover"`
	DisablePositionRecover    bool `json:"disablePositionRecover"`

	// EnableQuoteInvestmentReallocate set to true, the quote investment will be reallocated when the notional or quantity is under minimum.
	EnableQuoteInvestmentReallocate bool `json:"enableQuoteInvestmentReallocate"`

	// KeepOrdersWhenShutdown option is used for keeping the grid orders when shutting down bbgo
	KeepOrdersWhenShutdown bool `json:"keepOrdersWhenShutdown"`

	// UseCancelAllOrdersApiWhenClose close all orders even though the orders don't belong to this strategy
	UseCancelAllOrdersApiWhenClose bool `json:"useCancelAllOrdersApiWhenClose"`

	// log
	logger    *logrus.Entry
	LogFields logrus.Fields `json:"logFields"`

	// PrometheusLabels will be used as the base prometheus labels
	PrometheusLabels prometheus.Labels `json:"prometheusLabels"`

	// private field
	mu                   sync.Mutex
	nextStateC           chan statemachine.State
	state                statemachine.State
	collector            *Collector
	takeProfitPrice      fixedpoint.Value
	startTimeOfNextRound time.Time
	nextRoundPaused      bool

	// state machine
	stateMachine *statemachine.StateMachine

	// writeMutex is used to protect submit/cancel orders
	writeMutex  sync.Mutex
	writeCtx    context.Context
	cancelWrite context.CancelFunc

	// callbacks
	common.StatusCallbacks
	profitCallbacks         []func(*ProfitStats)
	positionUpdateCallbacks []func(*types.Position)
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
		return fmt.Errorf("priceDeviation can not be <= 0")
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

	// context for submit/cancel orders
	s.writeCtx, s.cancelWrite = context.WithCancel(ctx)

	s.logger.Infof("persistence ttl: %s", s.PersistenceTTL.Duration())
	if s.ProfitStats == nil {
		s.ProfitStats = newProfitStats(s.Market, s.QuoteInvestment)
	}

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	// set ttl for persistence
	s.Position.SetTTL(s.PersistenceTTL.Duration())
	s.ProfitStats.SetTTL(s.PersistenceTTL.Duration())

	if s.OrderGroupID == 0 {
		s.OrderGroupID = util.FNV32(instanceID) % math.MaxInt32
	}

	// collector
	s.collector = NewCollector(s.logger, s.Symbol, s.OrderGroupID, !s.DisableOrderGroupIDFilter, s.ExchangeSession.Exchange)
	if s.collector == nil {
		return fmt.Errorf("failed to initialize collector")
	}

	// prometheus
	if s.PrometheusLabels != nil {
		initMetrics(labelKeys(s.PrometheusLabels))
	}
	registerMetrics()

	// prometheus labels
	baseLabels = s.newPrometheusLabels()

	s.Position.Strategy = ID
	s.Position.StrategyInstanceID = instanceID

	if session.MakerFeeRate.Sign() > 0 || session.TakerFeeRate.Sign() > 0 {
		s.Position.SetExchangeFeeRate(session.ExchangeName, types.ExchangeFee{
			MakerFeeRate: session.MakerFeeRate,
			TakerFeeRate: session.TakerFeeRate,
		})
	}

	// state machine
	s.stateMachine = statemachine.NewStateMachine(s.logger)
	s.stateMachine.OnStart(func() {})
	s.stateMachine.OnClose(func() {})

	// order executor
	s.OrderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.OrderExecutor.SetLogger(s.logger)
	s.OrderExecutor.SetMaxRetries(5)
	s.OrderExecutor.BindEnvironment(s.Environment)
	s.OrderExecutor.Bind()

	// order executor
	s.OrderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		s.logger.Infof("POSITION UPDATE: %s", s.Position.String())
		bbgo.Sync(ctx, s)

		// update take profit price here
		s.updateTakeProfitPrice()

		// emit position update
		s.EmitPositionUpdate(position)
	})

	s.OrderExecutor.ActiveMakerOrders().OnFilled(func(o types.Order) {
		s.logger.Infof("FILLED ORDER: %s", o.String())

		switch o.Side {
		case OpenPositionSide:
			s.emitNextState(OpenPositionOrderFilled)
		case TakeProfitSide:
			s.emitNextState(IdleWaiting)
		default:
			s.logger.Infof("unsupported side (%s) of order: %s", o.Side, o)
		}

		openOrders, err := retry.QueryOpenOrdersUntilSuccessful(ctx, s.ExchangeSession.Exchange, s.Symbol)
		if err != nil {
			s.logger.WithError(err).Warn("failed to query open orders when order filled")
		}

		// update active orders metrics
		numActiveMakerOrders := s.OrderExecutor.ActiveMakerOrders().NumOfOrders()
		updateNumOfActiveOrdersMetrics(numActiveMakerOrders)

		if len(openOrders) != numActiveMakerOrders {
			s.logger.Warnf("num of open orders (%d) and active orders (%d) is different when order filled, please check it.", len(openOrders), numActiveMakerOrders)
		}

		if err == nil && o.Side == OpenPositionSide && numActiveMakerOrders == 0 && len(openOrders) == 0 {
			s.emitNextState(OpenPositionFinished)
		}
	})

	session.MarketDataStream.OnKLine(func(kline types.KLine) {
		switch s.state {
		case OpenPositionOrderFilled:
			if s.takeProfitPrice.IsZero() {
				s.logger.Warn("take profit price should not be 0 when there is at least one open-position order filled, please check it")
				return
			}

			compRes := kline.Close.Compare(s.takeProfitPrice)
			// price doesn't hit the take profit price
			if compRes < 0 {
				return
			}

			s.emitNextState(OpenPositionFinished)
		default:
			return
		}
	})

	session.UserDataStream.OnAuth(func() {
		s.logger.Info("user data stream authenticated")
		time.AfterFunc(3*time.Second, func() {
			if isInitialize := s.initializeNextStateC(); !isInitialize {

				// no need to recover when two situations if recoverWhenStart is false
				if !s.RecoverWhenStart {
					s.updateState(IdleWaiting)
				} else {
					// recover
					maxTry := 3
					for try := 1; try <= maxTry; try++ {
						s.logger.Infof("try #%d recover", try)

						err := s.recover(ctx)
						if err == nil {
							s.logger.Infof("recover successfully at #%d", try)
							break
						}

						s.logger.WithError(err).Warnf("failed to recover at #%d", try)

						if try == 3 {
							s.logger.Errorf("failed to recover after %d trying, please check it", maxTry)
							return
						}

						// sleep 10 second to retry the recovery
						time.Sleep(10 * time.Second)
					}
				}

				s.logger.Infof("state: %d", s.state)
				s.logger.Infof("position %s", s.Position.String())
				s.logger.Infof("profit stats %s", s.ProfitStats.String())
				s.logger.Infof("startTimeOfNextRound %s", s.startTimeOfNextRound)

				// emit position after recovery
				s.OrderExecutor.TradeCollector().EmitPositionUpdate(s.Position)

				s.updateTakeProfitPrice()

				// store persistence
				bbgo.Sync(ctx, s)

				// ready
				s.EmitReady()

				// start to sync periodically
				go s.syncPeriodically(ctx)

				// try to trigger position opening immediately
				if s.state == IdleWaiting {
					s.emitNextState(OpenPositionReady)
				}

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
			s.logger.WithError(err).Errorf("failed to close dca2 gracefully")
		}
	})

	return nil
}

func (s *Strategy) updateTakeProfitPrice() {
	takeProfitRatio := s.TakeProfitRatio
	s.takeProfitPrice = s.Market.TruncatePrice(s.Position.AverageCost.Mul(fixedpoint.One.Add(takeProfitRatio)))
	s.logger.Infof("cost: %s, ratio: %s, price: %s", s.Position.AverageCost.String(), takeProfitRatio.String(), s.takeProfitPrice.String())
}

func (s *Strategy) Close(ctx context.Context) error {
	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()

	s.logger.Infof("closing %s dca2", s.Symbol)

	// cancel write context to stop the order executor
	if s.cancelWrite != nil {
		s.cancelWrite()
	}

	defer s.EmitClosed()

	var err error
	if s.UseCancelAllOrdersApiWhenClose {
		err = tradingutil.UniversalCancelAllOrders(ctx, s.ExchangeSession.Exchange, s.Symbol, nil)
	} else {
		err = s.OrderExecutor.GracefulCancel(ctx)
	}

	if err != nil {
		s.logger.WithError(err).Errorf("there are errors when cancelling orders when closing (UseCancelAllOrdersApiWhenClose = %t)", s.UseCancelAllOrdersApiWhenClose)
	}

	bbgo.Sync(ctx, s)
	return err
}

func (s *Strategy) CleanUp(ctx context.Context) error {
	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()

	_ = s.Initialize()

	s.logger.Infof("cleaning up %s dca2", s.Symbol)

	// cancel write context to stop the order executor
	if s.cancelWrite != nil {
		s.cancelWrite()
	}

	defer s.EmitClosed()

	session := s.ExchangeSession
	if session == nil {
		return fmt.Errorf("session is nil, please check it")
	}

	// ignore the first cancel error, this skips one open-orders query request
	if err := tradingutil.UniversalCancelAllOrders(ctx, session.Exchange, s.Symbol, nil); err == nil {
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

		if err := tradingutil.UniversalCancelAllOrders(ctx, session.Exchange, s.Symbol, openOrders); err != nil {
			s.logger.WithError(err).Errorf("unable to cancel all orders")
			werr = multierr.Append(werr, err)
		}

		time.Sleep(1 * time.Second)
	}

	return werr
}

// PauseNextRound will stop openning open-position orders at the next round
func (s *Strategy) PauseNextRound() {
	s.nextRoundPaused = true
}

func (s *Strategy) ContinueNextRound() {
	s.nextRoundPaused = false
}

func (s *Strategy) UpdateProfitStatsUntilSuccessful(ctx context.Context) error {
	var op = func() error {
		if updated, err := s.UpdateProfitStats(ctx); err != nil {
			return errors.Wrapf(err, "failed to update profit stats, please check it")
		} else if !updated {
			return fmt.Errorf("there is no round to update profit stats, please check it")
		}

		return nil
	}

	// exponential increased interval retry until success
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 5 * time.Second
	bo.MaxInterval = 20 * time.Minute
	bo.MaxElapsedTime = 0

	return backoff.Retry(op, backoff.WithContext(bo, ctx))
}

// UpdateProfitStats will collect round from closed orders and emit update profit stats
// return true, nil -> there is at least one finished round and all the finished rounds we collect update profit stats successfully
// return false, nil -> there is no finished round!
// return true, error -> At least one round update profit stats successfully but there is error when collecting other rounds
func (s *Strategy) UpdateProfitStats(ctx context.Context) (bool, error) {
	s.logger.Info("update profit stats")
	rounds, err := s.collector.CollectFinishRounds(ctx, s.ProfitStats.FromOrderID)
	if err != nil {
		return false, errors.Wrapf(err, "failed to collect finish rounds from #%d", s.ProfitStats.FromOrderID)
	}

	var updated bool = false
	for _, round := range rounds {
		trades, err := s.collector.CollectRoundTrades(ctx, round)
		if err != nil {
			return updated, errors.Wrapf(err, "failed to collect the trades of round")
		}

		for _, trade := range trades {
			s.logger.Infof("update profit stats from trade: %s", trade.String())
			s.ProfitStats.AddTrade(trade)
		}

		// update profit stats FromOrderID to make sure we will not collect duplicated rounds
		for _, order := range round.TakeProfitOrders {
			if order.OrderID >= s.ProfitStats.FromOrderID {
				s.ProfitStats.FromOrderID = order.OrderID + 1
			}
		}

		// update quote investment
		s.ProfitStats.QuoteInvestment = s.ProfitStats.QuoteInvestment.Add(s.ProfitStats.CurrentRoundProfit)

		// sync to persistence
		bbgo.Sync(ctx, s)
		updated = true

		s.logger.Infof("profit stats:\n%s", s.ProfitStats.String())

		// emit profit
		s.EmitProfit(s.ProfitStats)

		// make profit stats forward to new round
		s.ProfitStats.NewRound()
	}

	return updated, nil
}

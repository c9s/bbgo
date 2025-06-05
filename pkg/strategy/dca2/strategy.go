package dca2

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/strategy/dca2/statemachine"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
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
	collector            *Collector
	takeProfitPrice      fixedpoint.Value
	startTimeOfNextRound time.Time
	nextRoundPaused      bool

	// state machine
	stateMachine *statemachine.StateMachine

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

	// order executor
	s.OrderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.OrderExecutor.SetLogger(s.logger)
	s.OrderExecutor.SetMaxRetries(5)
	s.OrderExecutor.BindEnvironment(s.Environment)
	s.OrderExecutor.Bind()

	session.UserDataStream.OnAuth(func() {
		s.logger.Info("user data stream authenticated")
		time.AfterFunc(3*time.Second, func() {
			s.stateMachine.Run(ctx)
		})
	})

	// set price trigger mode
	s.SetupPriceTriggerMode(ctx)

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
	s.logger.Infof("closing %s dca2", s.Symbol)

	defer s.EmitClosed()

	// this is async call, we need to wait for the state machine to close
	s.stateMachine.Close()

	// wait for the state machine to close, timeout after 15 seconds and check every 100 milliseconds
	checkInterval := 100 * time.Millisecond
	timeout := 15 * time.Second
	if isClosed := s.stateMachine.WaitForRunningIs(false, checkInterval, timeout); !isClosed {
		s.logger.Infof("state machine for %s dca2 is still not closed after %s, please check it", s.Symbol, timeout)
		return fmt.Errorf("state machine for %s dca2 is still not closed after %s, please check it", s.Symbol, timeout)
	}

	bbgo.Sync(ctx, s)
	return nil
}

func (s *Strategy) CleanUp(ctx context.Context) error {
	s.logger.Infof("cleaning up %s dca2", s.Symbol)
	_ = s.Initialize()

	defer s.EmitClosed()

	// this is async call, we need to wait for the state machine to close
	s.stateMachine.Close()

	// wait for the state machine to close, timeout after 15 seconds and check every 100 milliseconds
	checkInterval := 100 * time.Millisecond
	timeout := 15 * time.Second
	if isClosed := s.stateMachine.WaitForRunningIs(false, checkInterval, timeout); !isClosed {
		s.logger.Infof("state machine for %s dca2 is still not closed after %s, please check it", s.Symbol, timeout)
		return fmt.Errorf("state machine for %s dca2 is still not closed after %s, please check it", s.Symbol, timeout)
	}

	return nil
}

// PauseNextRound will stop openning open-position orders at the next round
func (s *Strategy) PauseNextRound() {
	s.nextRoundPaused = true
}

func (s *Strategy) ContinueNextRound() {
	s.nextRoundPaused = false
}

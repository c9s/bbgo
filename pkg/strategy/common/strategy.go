package common

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/risk/riskcontrol"
	"github.com/c9s/bbgo/pkg/types"
)

type RiskController struct {
	PositionHardLimit         fixedpoint.Value     `json:"positionHardLimit"`
	MaxPositionQuantity       fixedpoint.Value     `json:"maxPositionQuantity"`
	CircuitBreakLossThreshold fixedpoint.Value     `json:"circuitBreakLossThreshold"`
	CircuitBreakEMA           types.IntervalWindow `json:"circuitBreakEMA"`

	positionRiskControl     *riskcontrol.PositionRiskControl
	circuitBreakRiskControl *riskcontrol.CircuitBreakRiskControl
}

// Strategy provides the core functionality that is required by a long/short strategy.
type Strategy struct {
	Position    *types.Position    `json:"position,omitempty" persistence:"position"`
	ProfitStats *types.ProfitStats `json:"profitStats,omitempty" persistence:"profit_stats"`

	parent, ctx context.Context
	cancel      context.CancelFunc

	Environ       *bbgo.Environment
	Session       *bbgo.ExchangeSession
	OrderExecutor *bbgo.GeneralOrderExecutor

	RiskController
}

// TODO: use this to replace the parameters
type StrategyInstance interface {
	ID() string
	InstanceID() string
}

func NewStrategy(ctx context.Context, environ *bbgo.Environment, session *bbgo.ExchangeSession, market types.Market, strategyID, instanceID string) *Strategy {
	s := &Strategy{
		Environ: environ,
		Session: session,
	}
	s.Initialize(ctx, environ, session, market, strategyID, instanceID)
	return s
}

func (s *Strategy) Initialize(ctx context.Context, environ *bbgo.Environment, session *bbgo.ExchangeSession, market types.Market, strategyID, instanceID string) {
	s.parent = ctx
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.Environ = environ
	s.Session = session

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(market)
	}

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(market)
	}

	// Always update the position fields
	s.Position.Strategy = strategyID
	s.Position.StrategyInstanceID = instanceID

	// if anyone of the fee rate is defined, this assumes that both are defined.
	// so that zero maker fee could be applied
	if session.MakerFeeRate.Sign() > 0 || session.TakerFeeRate.Sign() > 0 {
		s.Position.SetExchangeFeeRate(session.ExchangeName, types.ExchangeFee{
			MakerFeeRate: session.MakerFeeRate,
			TakerFeeRate: session.TakerFeeRate,
		})
	}

	s.OrderExecutor = bbgo.NewGeneralOrderExecutor(session, market.Symbol, strategyID, instanceID, s.Position)
	s.OrderExecutor.BindEnvironment(environ)
	s.OrderExecutor.BindProfitStats(s.ProfitStats)
	s.OrderExecutor.Bind()
	s.OrderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		// bbgo.Sync(ctx, s)
	})

	if environ.GoogleSpreadSheetService != nil {
		// allocate a google spread sheet for this strategy
	}

	if !s.PositionHardLimit.IsZero() && !s.MaxPositionQuantity.IsZero() {
		log.Infof("positionHardLimit and maxPositionQuantity are configured, setting up PositionRiskControl...")
		s.positionRiskControl = riskcontrol.NewPositionRiskControl(s.OrderExecutor, s.PositionHardLimit, s.MaxPositionQuantity)
	}

	if !s.CircuitBreakLossThreshold.IsZero() {
		log.Infof("circuitBreakLossThreshold is configured, setting up CircuitBreakRiskControl...")
		s.circuitBreakRiskControl = riskcontrol.NewCircuitBreakRiskControl(
			s.Position,
			session.Indicators(market.Symbol).EWMA(s.CircuitBreakEMA),
			s.CircuitBreakLossThreshold,
			s.ProfitStats)
	}
}

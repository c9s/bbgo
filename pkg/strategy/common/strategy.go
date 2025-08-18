package common

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/risk/circuitbreaker"
	"github.com/c9s/bbgo/pkg/risk/riskcontrol"
	"github.com/c9s/bbgo/pkg/types"
)

type RiskController struct {
	PositionHardLimit         fixedpoint.Value `json:"positionHardLimit"`
	MaxPositionQuantity       fixedpoint.Value `json:"maxPositionQuantity"`
	CircuitBreakLossThreshold fixedpoint.Value `json:"circuitBreakLossThreshold"`

	positionRiskControl     *riskcontrol.PositionRiskControl
	circuitBreakRiskControl *circuitbreaker.BasicCircuitBreaker
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

func (s *Strategy) Initialize(
	ctx context.Context, environ *bbgo.Environment, session *bbgo.ExchangeSession, market types.Market, strategyID, instanceID string,
) {
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

	if !s.PositionHardLimit.IsZero() && !s.MaxPositionQuantity.IsZero() {
		log.Infof("positionHardLimit and maxPositionQuantity are configured, setting up PositionRiskControl...")
		s.positionRiskControl = riskcontrol.NewPositionRiskControl(s.OrderExecutor, s.PositionHardLimit, s.MaxPositionQuantity)
		s.positionRiskControl.Initialize(ctx, session)
	}

	if !s.CircuitBreakLossThreshold.IsZero() {
		log.Infof("circuitBreakLossThreshold is configured, setting up CircuitBreakRiskControl...")
		s.circuitBreakRiskControl = &circuitbreaker.BasicCircuitBreaker{
			Enabled:          true,
			MaximumTotalLoss: s.CircuitBreakLossThreshold,
			HaltDuration:     types.Duration(24 * time.Hour),
		}
		s.circuitBreakRiskControl.SetMetricsInfo(strategyID, instanceID, market.Symbol)

		s.OrderExecutor.TradeCollector().OnProfit(func(trade types.Trade, profit *types.Profit) {
			if profit != nil && s.circuitBreakRiskControl != nil {
				s.circuitBreakRiskControl.RecordProfit(profit.Profit, trade.Time.Time())
			}
		})
	}
}

func (s *Strategy) IsHalted(t time.Time) bool {
	if s.circuitBreakRiskControl == nil {
		return false
	}

	_, isHalted := s.circuitBreakRiskControl.IsHalted(t)
	return isHalted
}

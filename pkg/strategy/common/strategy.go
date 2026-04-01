package common

import (
	"context"
	"encoding/json"
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
	TradeStats  *types.TradeStats  `json:"tradeStats,omitempty" persistence:"trade_stats"`

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

	if s.TradeStats == nil {
		s.TradeStats = types.NewTradeStats(market.Symbol)
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
	s.OrderExecutor.BindTradeStats(s.TradeStats)
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

type TabularStats interface {
	SummaryHeader() []string
	SummaryRecords() [][]string
}

// StrategySummarizer provides a convenient way for strategies to export detailed performance metrics
// in tabular (CSV) and JSON formats. This interface is designed to produce structured, human-readable
// summaries of strategy performance, useful for inspection and post-analysis. It differs from the
// backtest module's StateRecorder interface, which is primarily a logger that records state transitions
// over time. TabularStats formats comprehensive performance data (e.g., trade statistics, profit
// analysis, position summaries) as structured tables, while json.Marshaler entries provide the same
// or additional data in JSON form. It can also export time series metrics with varying time
// granularities (e.g., daily, hourly, 1m metrics), with each time series represented as a separate
// entry in the returned maps.
type StrategySummarizer interface {
	// SummaryStats returns two maps keyed by logical grouping names
	// (e.g., "trades", "positions", "profits", "daily_metrics", "hourly_metrics").
	// The first map contains TabularStats for CSV/tabular output, and the second contains
	// json.Marshaler instances for JSON output. Keys are typically used as output file names
	// or section identifiers. Time series metrics with different granularities should each
	// have their own entry in the maps.
	SummaryStats() (map[string]TabularStats, map[string]json.Marshaler)
}

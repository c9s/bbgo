package riskcontrol

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
	log "github.com/sirupsen/logrus"
)

type CircuitBreakRiskControl struct {
	// Since price could be fluctuated large,
	// use an EWMA to smooth it in running time
	price          *indicator.EWMA
	position       *types.Position
	profitStats    *types.ProfitStats
	breakCondition fixedpoint.Value
}

func NewCircuitBreakRiskControl(
	position *types.Position,
	price *indicator.EWMA,
	breakCondition fixedpoint.Value,
	profitStats *types.ProfitStats) *CircuitBreakRiskControl {

	return &CircuitBreakRiskControl{
		price:          price,
		position:       position,
		profitStats:    profitStats,
		breakCondition: breakCondition,
	}
}

// IsHalted returns whether we reached the circuit break condition set for this day?
func (c *CircuitBreakRiskControl) IsHalted() bool {
	var unrealized = c.position.UnrealizedProfit(fixedpoint.NewFromFloat(c.price.Last(0)))
	log.Infof("[CircuitBreakRiskControl] Realized P&L = %v, Unrealized P&L = %v\n",
		c.profitStats.TodayPnL,
		unrealized)
	return unrealized.Add(c.profitStats.TodayPnL).Compare(c.breakCondition) <= 0
}

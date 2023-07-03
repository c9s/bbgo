package riskcontrol

import (
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

type CircuitBreakRiskControl struct {
	// Since price could be fluctuated large,
	// use an EWMA to smooth it in running time
	price         *indicator.EWMAStream
	position      *types.Position
	profitStats   *types.ProfitStats
	lossThreshold fixedpoint.Value
}

func NewCircuitBreakRiskControl(
	position *types.Position,
	price *indicator.EWMAStream,
	lossThreshold fixedpoint.Value,
	profitStats *types.ProfitStats) *CircuitBreakRiskControl {

	return &CircuitBreakRiskControl{
		price:         price,
		position:      position,
		profitStats:   profitStats,
		lossThreshold: lossThreshold,
	}
}

// IsHalted returns whether we reached the circuit break condition set for this day?
func (c *CircuitBreakRiskControl) IsHalted() bool {
	var unrealized = c.position.UnrealizedProfit(fixedpoint.NewFromFloat(c.price.Last(0)))
	log.Infof("[CircuitBreakRiskControl] realized PnL = %f, unrealized PnL = %f\n",
		c.profitStats.TodayPnL.Float64(),
		unrealized.Float64())
	return unrealized.Add(c.profitStats.TodayPnL).Compare(c.lossThreshold) <= 0
}

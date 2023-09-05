package riskcontrol

import (
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type CircuitBreakRiskControl struct {
	// Since price could be fluctuated large,
	// use an EWMA to smooth it in running time
	price          *indicatorv2.EWMAStream
	position       *types.Position
	profitStats    *types.ProfitStats
	lossThreshold  fixedpoint.Value
	haltedDuration time.Duration

	isHalted bool
	haltedAt time.Time
}

func NewCircuitBreakRiskControl(
	position *types.Position,
	price *indicatorv2.EWMAStream,
	lossThreshold fixedpoint.Value,
	profitStats *types.ProfitStats,
	haltedDuration time.Duration,
) *CircuitBreakRiskControl {
	return &CircuitBreakRiskControl{
		price:          price,
		position:       position,
		profitStats:    profitStats,
		lossThreshold:  lossThreshold,
		haltedDuration: haltedDuration,
	}
}

func (c *CircuitBreakRiskControl) IsOverHaltedDuration() bool {
	return time.Since(c.haltedAt) >= c.haltedDuration
}

// IsHalted returns whether we reached the circuit break condition set for this day?
func (c *CircuitBreakRiskControl) IsHalted(t time.Time) bool {
	if c.profitStats.IsOver24Hours() {
		c.profitStats.ResetToday(t)
	}

	// if we are not over the halted duration, we don't need to check the condition
	if !c.IsOverHaltedDuration() {
		return false
	}

	var unrealized = c.position.UnrealizedProfit(fixedpoint.NewFromFloat(c.price.Last(0)))
	log.Infof("[CircuitBreakRiskControl] realized PnL = %f, unrealized PnL = %f\n",
		c.profitStats.TodayPnL.Float64(),
		unrealized.Float64())

	c.isHalted = unrealized.Add(c.profitStats.TodayPnL).Compare(c.lossThreshold) <= 0
	if c.isHalted {
		c.haltedAt = t
	}

	return c.isHalted
}

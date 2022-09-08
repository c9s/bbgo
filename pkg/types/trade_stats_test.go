package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func number(v float64) fixedpoint.Value {
	return fixedpoint.NewFromFloat(v)
}

func TestTradeStats_consecutiveCounterAndAmount(t *testing.T) {
	stats := NewTradeStats("BTCUSDT")
	stats.add(&Profit{OrderID: 1, Profit: number(20.0)})
	stats.add(&Profit{OrderID: 1, Profit: number(30.0)})

	assert.Equal(t, 1, stats.consecutiveSide)
	assert.Equal(t, 1, stats.consecutiveCounter)
	assert.Equal(t, "50", stats.consecutiveAmount.String())

	stats.add(&Profit{OrderID: 2, Profit: number(50.0)})
	stats.add(&Profit{OrderID: 2, Profit: number(50.0)})
	assert.Equal(t, 1, stats.consecutiveSide)
	assert.Equal(t, 2, stats.consecutiveCounter)
	assert.Equal(t, "150", stats.consecutiveAmount.String())
	assert.Equal(t, 2, stats.MaximumConsecutiveWins)

	stats.add(&Profit{OrderID: 3, Profit: number(-50.0)})
	stats.add(&Profit{OrderID: 3, Profit: number(-50.0)})
	assert.Equal(t, -1, stats.consecutiveSide)
	assert.Equal(t, 1, stats.consecutiveCounter)
	assert.Equal(t, "-100", stats.consecutiveAmount.String())

	assert.Equal(t, "150", stats.MaximumConsecutiveProfit.String())
	assert.Equal(t, "0", stats.MaximumConsecutiveLoss.String())

	stats.add(&Profit{OrderID: 4, Profit: number(-100.0)})
	assert.Equal(t, -1, stats.consecutiveSide)
	assert.Equal(t, 2, stats.consecutiveCounter)
	assert.Equal(t, "-200", stats.MaximumConsecutiveLoss.String())
	assert.Equal(t, 2, stats.MaximumConsecutiveLosses)
}

package common

import (
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestFeeBudget(t *testing.T) {
	cases := []struct {
		budgets  map[string]fixedpoint.Value
		trades   []types.Trade
		expected bool
	}{
		{
			budgets: map[string]fixedpoint.Value{
				"MAX": fixedpoint.NewFromFloat(0.5),
			},
			trades: []types.Trade{
				{FeeCurrency: "MAX", Fee: fixedpoint.NewFromFloat(0.1)},
				{FeeCurrency: "USDT", Fee: fixedpoint.NewFromFloat(10.0)},
			},
			expected: true,
		},
		{
			budgets: map[string]fixedpoint.Value{
				"MAX": fixedpoint.NewFromFloat(0.5),
			},
			trades: []types.Trade{
				{FeeCurrency: "MAX", Fee: fixedpoint.NewFromFloat(0.1)},
				{FeeCurrency: "MAX", Fee: fixedpoint.NewFromFloat(0.5)},
				{FeeCurrency: "USDT", Fee: fixedpoint.NewFromFloat(10.0)},
			},
			expected: false,
		},
	}

	for _, c := range cases {
		feeBudget := FeeBudget{
			DailyFeeBudgets: c.budgets,
		}
		feeBudget.Initialize()

		for _, trade := range c.trades {
			feeBudget.HandleTradeUpdate(trade)
		}
		assert.Equal(t, c.expected, feeBudget.IsBudgetAllowed())

		// test reset
		feeBudget.State.AccumulatedFeeStartedAt = feeBudget.State.AccumulatedFeeStartedAt.Add(-24 * time.Hour)
		assert.True(t, feeBudget.IsBudgetAllowed())
	}
}

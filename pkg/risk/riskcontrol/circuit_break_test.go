package riskcontrol

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_IsHalted(t *testing.T) {
	var (
		price          = 30000.00
		realizedPnL    = fixedpoint.NewFromFloat(-100.0)
		breakCondition = fixedpoint.NewFromFloat(-500.00)
	)

	window := types.IntervalWindow{Window: 30, Interval: types.Interval1m}
	priceEWMA := indicatorv2.EWMA2(nil, window.Window)
	priceEWMA.PushAndEmit(price)

	cases := []struct {
		name        string
		position    fixedpoint.Value
		averageCost fixedpoint.Value
		isHalted    bool
	}{
		{
			name:        "PositivePositionReachBreakCondition",
			position:    fixedpoint.NewFromFloat(10.0),
			averageCost: fixedpoint.NewFromFloat(30040.0),
			isHalted:    true,
		}, {
			name:        "PositivePositionOverBreakCondition",
			position:    fixedpoint.NewFromFloat(10.0),
			averageCost: fixedpoint.NewFromFloat(30050.0),
			isHalted:    true,
		}, {
			name:        "PositivePositionUnderBreakCondition",
			position:    fixedpoint.NewFromFloat(10.0),
			averageCost: fixedpoint.NewFromFloat(30030.0),
			isHalted:    false,
		}, {
			name:        "NegativePositionReachBreakCondition",
			position:    fixedpoint.NewFromFloat(-10.0),
			averageCost: fixedpoint.NewFromFloat(29960.0),
			isHalted:    true,
		}, {
			name:        "NegativePositionOverBreakCondition",
			position:    fixedpoint.NewFromFloat(-10.0),
			averageCost: fixedpoint.NewFromFloat(29950.0),
			isHalted:    true,
		}, {
			name:        "NegativePositionUnderBreakCondition",
			position:    fixedpoint.NewFromFloat(-10.0),
			averageCost: fixedpoint.NewFromFloat(29970.0),
			isHalted:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var riskControl = NewCircuitBreakRiskControl(
				&types.Position{
					Base:        tc.position,
					AverageCost: tc.averageCost,
				},
				priceEWMA,
				breakCondition,
				&types.ProfitStats{},
				24*time.Hour,
			)
			now := time.Now()
			riskControl.profitStats.ResetToday(now)
			riskControl.profitStats.TodayPnL = realizedPnL
			assert.Equal(t, tc.isHalted, riskControl.IsHalted(now.Add(time.Hour)))
		})
	}
}

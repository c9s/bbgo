package util

import "github.com/c9s/bbgo/pkg/fixedpoint"

// CalculateMarginTolerance calculates the margin tolerance ranges from 0.0 (liquidation) to 1.0 (safest level of margin).
func CalculateMarginTolerance(marginLevel fixedpoint.Value) fixedpoint.Value {
	if marginLevel.IsZero() {
		// Although margin level shouldn't be zero, that would indicate a significant problem.
		// In that case, margin tolerance should return 0.0 to also reflect that problem.
		return fixedpoint.Zero
	}

	// Formula created by operations team for our binance code.  Liquidation occurs at 1.1,
	// so when marginLevel equals 1.1, the formula becomes 1.0 - 1.0, or zero.
	// = 1.0 - (1.1 / marginLevel)
	return fixedpoint.One.Sub(fixedpoint.NewFromFloat(1.1).Div(marginLevel))
}

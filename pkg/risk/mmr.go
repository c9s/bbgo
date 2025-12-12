package risk

import "github.com/c9s/bbgo/pkg/fixedpoint"

// DefaultMaintenanceMarginRatio calculates the default maintenance margin ratio based on leverage.
func DefaultMaintenanceMarginRatio(leverage fixedpoint.Value) fixedpoint.Value {
	defaultMmr := fixedpoint.NewFromFloat(9.0 * 0.01)
	if leverage.Compare(fixedpoint.NewFromFloat(10.0)) >= 0 {
		defaultMmr = fixedpoint.NewFromFloat(5.0 * 0.01) // 5%
	} else if leverage.Compare(fixedpoint.NewFromFloat(5.0)) >= 0 {
		defaultMmr = fixedpoint.NewFromFloat(9.0 * 0.01) // 9%
	} else if leverage.Compare(fixedpoint.NewFromFloat(3.0)) >= 0 {
		defaultMmr = fixedpoint.NewFromFloat(10.0 * 0.01) // 10%
	}
	return defaultMmr
}

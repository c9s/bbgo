package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// n is a shorthand for building the fixedpoint prices of the test klines.
func n(v float64) fixedpoint.Value {
	return fixedpoint.NewFromFloat(v)
}

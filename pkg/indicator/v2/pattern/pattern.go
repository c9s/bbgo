package pattern

import "github.com/c9s/bbgo/pkg/fixedpoint"

var (
	Neutral   = .0
	Bull      = 1.0
	Bear      = -1.0
	threshold = fixedpoint.NewFromFloat(0.1)
	limit     = fixedpoint.NewFromFloat(0.2)
)

func n(n float64) fixedpoint.Value {
	return fixedpoint.NewFromFloat(n)
}

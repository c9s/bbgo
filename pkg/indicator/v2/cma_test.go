package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func Test_CMA2(t *testing.T) {
	source := types.NewFloat64Series()
	cma := CMA2(source)

	n := MaxSliceSize + 10
	for i := 1; i <= n; i++ {
		source.PushAndEmit(float64(i))
	}

	// the output slice is truncated, but the cumulative average still covers all inputs
	assert.Less(t, cma.Length(), n)
	assert.InDelta(t, float64(n+1)/2.0, cma.Last(0), 1e-9)
}

package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestEWMA2_ZeroValue(t *testing.T) {
	source := types.NewFloat64Series()
	ewma := EWMA2(source, 3)

	// a zero value must be smoothed like any other value instead of re-seeding the average
	for _, v := range []float64{0, 4, 8} {
		source.PushAndEmit(v)
	}

	// multiplier = 2 / (1 + 3) = 0.5
	assert.Equal(t, []float64{0, 2, 5}, []float64(ewma.Slice))
}

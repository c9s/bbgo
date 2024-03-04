package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestSMMA(t *testing.T) {
	source := types.NewFloat64Series()
	smma := SMMA2(source, 3)

	data := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90}
	for _, d := range data {
		source.PushAndEmit(d)
	}

	// Assert the first 3 and last 3 value outputs.
	assert.InDelta(t, 0, smma.Last(len(data)-1), 0.001)
	assert.InDelta(t, 0, smma.Last(len(data)-2), 0.001)
	assert.InDelta(t, 20, smma.Last(len(data)-3), 0.001)
	assert.InDelta(t, 51.97530864197531, smma.Last(2), 0.001)
	assert.InDelta(t, 61.31687242798355, smma.Last(1), 0.001)
	assert.InDelta(t, 70.87791495198904, smma.Last(0), 0.001)
}

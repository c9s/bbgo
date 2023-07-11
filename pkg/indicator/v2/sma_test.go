package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestSMA(t *testing.T) {
	source := types.NewFloat64Series()
	sma := SMA(source, 9)

	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}
	for _, d := range data {
		source.PushAndEmit(d)
	}

	assert.InDelta(t, 5, sma.Last(0), 0.001)
	assert.InDelta(t, 4.5, sma.Last(1), 0.001)
}

package volatility

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestUlcerIndex(t *testing.T) {
	closing := []float64{9, 11, 7, 10, 8, 7, 7, 8, 10, 9, 5, 4, 6, 7}
	expected := []float64{0, 0, 20.99, 18.74, 20.73, 24.05, 26.17, 26.31,
		24.99, 24.39, 28.49, 32.88, 34.02, 34.19}

	source := types.NewFloat64Series()
	ind := UlcerIndexDefault(source)

	for _, d := range closing {
		source.PushAndEmit(d)
	}
	assert.InDeltaSlice(t, expected, ind.Slice, 0.01)
}

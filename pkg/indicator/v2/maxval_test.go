package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestMaximumValueIndicator(t *testing.T) {
	t.Run("rolling max series", func(t *testing.T) {
		data := []float64{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
		// expected := []float64{10, 10, 10, 10, 9, 8, 7, 6, 5, 4}

		source := types.NewFloat64Series()
		ind := MaxValue(source, 4)

		for _, d := range data {
			source.PushAndEmit(d)
		}
		assert.Equal(t, 4.0, ind.Last(0))
	})
}

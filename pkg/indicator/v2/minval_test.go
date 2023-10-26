package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestMinimumValueIndicator(t *testing.T) {
	t.Run("rolling min series", func(t *testing.T) {
		data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
		// expected := []float64{1, 1, 1, 1, 2, 3, 4, 5, 6, 7}

		source := types.NewFloat64Series()
		ind := MinValue(source, 4)

		for _, d := range data {
			source.PushAndEmit(d)
		}
		assert.Equal(t, 7.0, ind.Last(0))
	})
}

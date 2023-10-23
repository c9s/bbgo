package trend

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestAroonUpIndicator(t *testing.T) {
	t.Run("with < window periods", func(t *testing.T) {
		source := types.NewFloat64Series()
		aroonUp := AroonUpIndicator(source, 10)

		data := []float64{1}
		for _, d := range data {
			source.PushAndEmit(d)
		}
		assert.Equal(t, 0.0, aroonUp.Last(0))
	})

	t.Run("with > window periods", func(t *testing.T) {
		source := types.NewFloat64Series()
		aroonUp := AroonUpIndicator(source, 4)

		data := []float64{1, 2, 3, 4, 3, 2, 1}
		for _, d := range data {
			source.PushAndEmit(d)
		}
		assert.Equal(t, 100.0, aroonUp.Last(3))
		assert.Equal(t, 75.0, aroonUp.Last(4))
		assert.Equal(t, 50.0, aroonUp.Last(5))
	})
}

func TestAroonDownIndicator(t *testing.T) {
	t.Run("with < window periods", func(t *testing.T) {
		source := types.NewFloat64Series()
		aroonDown := AroonDownIndicator(source, 10)

		data := []float64{1}
		for _, d := range data {
			source.PushAndEmit(d)
		}
		assert.Equal(t, 0.0, aroonDown.Last(0))
	})

	t.Run("with > window periods", func(t *testing.T) {
		source := types.NewFloat64Series()
		aroonDown := AroonDownIndicator(source, 4)

		data := []float64{5, 4, 3, 2, 3, 4, 5}
		for _, d := range data {
			source.PushAndEmit(d)
		}
		assert.Equal(t, 100.0, aroonDown.Last(3))
		assert.Equal(t, 75.0, aroonDown.Last(4))
		assert.Equal(t, 50.0, aroonDown.Last(5))
	})
}

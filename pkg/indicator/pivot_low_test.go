package indicator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_calculatePivotLow(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		low, ok := calculatePivotLow([]float64{15.0, 13.0, 12.0, 10.0, 14.0, 15.0}, 2, 2)
		//                                          ^left ----- ^pivot ---- ^right
		assert.True(t, ok)
		assert.Equal(t, 10.0, low)

		low, ok = calculatePivotLow([]float64{15.0, 13.0, 12.0, 10.0, 14.0, 9.0}, 2, 2)
		//                                          ^left ----- ^pivot ---- ^right
		assert.False(t, ok)

		low, ok = calculatePivotLow([]float64{15.0, 9.0, 12.0, 10.0, 14.0, 15.0}, 2, 2)
		//                                          ^left ----- ^pivot ---- ^right
		assert.False(t, ok)
	})

	t.Run("different left and right", func(t *testing.T) {
		low, ok := calculatePivotLow([]float64{11.0, 12.0, 16.0, 15.0, 13.0, 12.0, 10.0, 14.0, 15.0}, 5, 2)
		//                                         ^left ---------------------- ^pivot ---- ^right

		assert.True(t, ok)
		assert.Equal(t, 10.0, low)

		low, ok = calculatePivotLow([]float64{9.0, 8.0, 16.0, 15.0, 13.0, 12.0, 10.0, 14.0, 15.0}, 5, 2)
		//                                         ^left ---------------------- ^pivot ---- ^right
		// 8.0 < 10.0
		assert.False(t, ok)
		assert.Equal(t, 0.0, low)
	})

	t.Run("right window 0", func(t *testing.T) {
		low, ok := calculatePivotLow([]float64{15.0, 13.0, 12.0, 10.0, 14.0, 15.0}, 2, 0)
		assert.True(t, ok)
		assert.Equal(t, 10.0, low)
	})

	t.Run("insufficient length", func(t *testing.T) {
		low, ok := calculatePivotLow([]float64{15.0, 13.0, 12.0, 10.0, 14.0, 15.0}, 3, 3)
		assert.False(t, ok)
		assert.Equal(t, 0.0, low)
	})

}

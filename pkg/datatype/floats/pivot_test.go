package floats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindPivot(t *testing.T) {

	t.Run("middle", func(t *testing.T) {
		pv, ok := FindPivot(Slice{10, 20, 30, 40, 30, 20}, 2, 2, func(a, pivot float64) bool {
			return a < pivot
		})
		if assert.True(t, ok) {
			assert.Equal(t, 40., pv)
		}
	})

	t.Run("last", func(t *testing.T) {
		pv, ok := FindPivot(Slice{10, 20, 30, 40, 30, 45}, 2, 0, func(a, pivot float64) bool {
			return a < pivot
		})
		if assert.True(t, ok) {
			assert.Equal(t, 45., pv)
		}
	})

}

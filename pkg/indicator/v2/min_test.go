package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestMIN(t *testing.T) {
	source := types.NewFloat64Series()
	minIndicator := MIN(source, 3)

	// Test case 1: Fill the window
	source.PushAndEmit(3.0)
	assert.Equal(t, 3.0, minIndicator.Last(0))

	source.PushAndEmit(1.0)
	assert.Equal(t, 1.0, minIndicator.Last(0))

	source.PushAndEmit(2.0)
	assert.Equal(t, 1.0, minIndicator.Last(0))

	// Test case 2: Sliding window
	source.PushAndEmit(3.0)
	// Window is [1.0, 2.0, 3.0], min is 1.0
	assert.Equal(t, 1.0, minIndicator.Last(0))

	source.PushAndEmit(4.0)
	// Window is [2.0, 3.0, 4.0], min is 2.0
	assert.Equal(t, 2.0, minIndicator.Last(0))

	source.PushAndEmit(0.5)
	// Window is [3.0, 4.0, 0.5], min is 0.5
	assert.Equal(t, 0.5, minIndicator.Last(0))
}

// TestMIN_ZeroValue covers a window that legitimately contains 0: a zero must
// not be treated as an empty slot.
func TestMIN_ZeroValue(t *testing.T) {
	source := types.NewFloat64Series()
	minIndicator := MIN(source, 5)

	for _, v := range []float64{0, 10, 20, 30, 40} {
		source.PushAndEmit(v)
	}
	assert.Equal(t, 0.0, minIndicator.Last(0))

	// 0 leaves the window
	source.PushAndEmit(50)
	assert.Equal(t, 10.0, minIndicator.Last(0))
}

package grid2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestNewGrid(t *testing.T) {
	upper := fixedpoint.NewFromFloat(500.0)
	lower := fixedpoint.NewFromFloat(100.0)
	size := fixedpoint.NewFromFloat(100.0)
	grid := NewGrid(lower, upper, size)
	assert.Equal(t, upper, grid.UpperPrice)
	assert.Equal(t, lower, grid.LowerPrice)
	assert.Equal(t, fixedpoint.NewFromFloat(4), grid.Spread)
	if assert.Len(t, grid.Pins, 101) {
		assert.Equal(t, fixedpoint.NewFromFloat(100.0), grid.Pins[0])
		assert.Equal(t, fixedpoint.NewFromFloat(500.0), grid.Pins[100])
	}
}

func TestGrid_HasPin(t *testing.T) {
	upper := fixedpoint.NewFromFloat(500.0)
	lower := fixedpoint.NewFromFloat(100.0)
	size := fixedpoint.NewFromFloat(100.0)
	grid := NewGrid(lower, upper, size)

	assert.True(t, grid.HasPin(fixedpoint.NewFromFloat(100.0)))
	assert.True(t, grid.HasPin(fixedpoint.NewFromFloat(500.0)))
	assert.False(t, grid.HasPin(fixedpoint.NewFromFloat(101.0)))
}

func TestGrid_ExtendUpperPrice(t *testing.T) {
	upper := fixedpoint.NewFromFloat(500.0)
	lower := fixedpoint.NewFromFloat(100.0)
	size := fixedpoint.NewFromFloat(100.0)
	grid := NewGrid(lower, upper, size)

	originalSpread := grid.Spread
	newPins := grid.ExtendUpperPrice(fixedpoint.NewFromFloat(1000.0))
	assert.Equal(t, originalSpread, grid.Spread)
	assert.Len(t, newPins, 125) // (1000-500) / 4
	assert.Equal(t, fixedpoint.NewFromFloat(4), grid.Spread)
	if assert.Len(t, grid.Pins, 226) {
		assert.Equal(t, fixedpoint.NewFromFloat(100.0), grid.Pins[0])
		assert.Equal(t, fixedpoint.NewFromFloat(1000.0), grid.Pins[225])
	}
}

func TestGrid_ExtendLowerPrice(t *testing.T) {
	upper := fixedpoint.NewFromFloat(3000.0)
	lower := fixedpoint.NewFromFloat(2000.0)
	size := fixedpoint.NewFromFloat(100.0)
	grid := NewGrid(lower, upper, size)

	// spread = (3000 - 2000) / 100.0
	expectedSpread := fixedpoint.NewFromFloat(10.0)
	assert.Equal(t, expectedSpread, grid.Spread)

	originalSpread := grid.Spread
	newPins := grid.ExtendLowerPrice(fixedpoint.NewFromFloat(1000.0))
	assert.Equal(t, originalSpread, grid.Spread)

	// 100 = (2000-1000) / 10
	if assert.Len(t, newPins, 100) {
		assert.Equal(t, fixedpoint.NewFromFloat(2000.0)-expectedSpread, newPins[99])
	}

	assert.Equal(t, expectedSpread, grid.Spread)
	if assert.Len(t, grid.Pins, 201) {
		assert.Equal(t, fixedpoint.NewFromFloat(1000.0), grid.Pins[0])
		assert.Equal(t, fixedpoint.NewFromFloat(3000.0), grid.Pins[200])
	}

	newPins2 := grid.ExtendLowerPrice(
		fixedpoint.NewFromFloat(1000.0 - 1.0))
	assert.Len(t, newPins2, 0) // should have no new pin generated
}

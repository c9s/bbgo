//go:build !dnum

package grid2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGrid_HasPrice(t *testing.T) {
	t.Run("case1", func(t *testing.T) {
		upper := number(500.0)
		lower := number(100.0)
		size := number(5.0)
		grid := NewGrid(lower, upper, size, number(0.01))
		grid.CalculateArithmeticPins()

		assert.True(t, grid.HasPrice(number(500.0)), "upper price")
		assert.True(t, grid.HasPrice(number(100.0)), "lower price")
		assert.True(t, grid.HasPrice(number(200.0)), "found 200 price ok")
		assert.True(t, grid.HasPrice(number(300.0)), "found 300 price ok")
	})

	t.Run("case2", func(t *testing.T) {
		upper := number(0.9)
		lower := number(0.1)
		size := number(7.0)
		grid := NewGrid(lower, upper, size, number(0.00000001))
		grid.CalculateArithmeticPins()

		assert.Equal(t, []Pin{
			Pin(number(0.1)),
			Pin(number(0.23333333)),
			Pin(number(0.36666666)),
			Pin(number(0.49999999)),
			Pin(number(0.63333332)),
			Pin(number(0.76666665)),
			Pin(number(0.9)),
		}, grid.Pins)

		assert.False(t, grid.HasPrice(number(200.0)), "out of range")
		assert.True(t, grid.HasPrice(number(0.9)), "upper price")
		assert.True(t, grid.HasPrice(number(0.1)), "lower price")
		assert.True(t, grid.HasPrice(number(0.49999999)), "found 0.49999999 price ok")
	})

	t.Run("case3", func(t *testing.T) {
		upper := number(0.9)
		lower := number(0.1)
		size := number(7.0)
		grid := NewGrid(lower, upper, size, number(0.0001))
		grid.CalculateArithmeticPins()

		assert.Equal(t, []Pin{
			Pin(number(0.1)),
			Pin(number(0.2333)),
			Pin(number(0.3666)),
			Pin(number(0.5000)),
			Pin(number(0.6333)),
			Pin(number(0.7666)),
			Pin(number(0.9)),
		}, grid.Pins)

		assert.False(t, grid.HasPrice(number(200.0)), "out of range")
		assert.True(t, grid.HasPrice(number(0.9)), "upper price")
		assert.True(t, grid.HasPrice(number(0.1)), "lower price")
		assert.True(t, grid.HasPrice(number(0.5)), "found 0.5 price ok")
		assert.True(t, grid.HasPrice(number(0.2333)), "found 0.2333 price ok")
	})

	t.Run("case4", func(t *testing.T) {
		upper := number(90.0)
		lower := number(10.0)
		size := number(7.0)
		grid := NewGrid(lower, upper, size, number(0.001))
		grid.CalculateArithmeticPins()

		assert.Equal(t, []Pin{
			Pin(number("10.0")),
			Pin(number("23.333")),
			Pin(number("36.666")),
			Pin(number("50.00")),
			Pin(number("63.333")),
			Pin(number("76.666")),
			Pin(number("90.0")),
		}, grid.Pins)

		assert.False(t, grid.HasPrice(number(200.0)), "out of range")
		assert.True(t, grid.HasPrice(number("36.666")), "found 36.666 price ok")
	})

}

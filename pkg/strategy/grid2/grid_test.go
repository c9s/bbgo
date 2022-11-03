package grid2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func number(a interface{}) fixedpoint.Value {
	if s, ok := a.(string); ok {
		return fixedpoint.MustNewFromString(s)
	}

	f := a.(float64)
	return fixedpoint.NewFromFloat(f)
}

func TestNewGrid(t *testing.T) {
	upper := fixedpoint.NewFromFloat(500.0)
	lower := fixedpoint.NewFromFloat(100.0)
	size := fixedpoint.NewFromFloat(100.0)
	grid := NewGrid(lower, upper, size, number(2.0))
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
	grid := NewGrid(lower, upper, size, number(2))

	assert.True(t, grid.HasPin(Pin(number(100.0))))
	assert.True(t, grid.HasPin(Pin(number(500.0))))
	assert.False(t, grid.HasPin(Pin(number(101.0))))
}

func TestGrid_ExtendUpperPrice(t *testing.T) {
	upper := number(500.0)
	lower := number(100.0)
	size := number(100.0)
	grid := NewGrid(lower, upper, size, number(2.0))

	originalSpread := grid.Spread
	newPins := grid.ExtendUpperPrice(number(1000.0))
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
	grid := NewGrid(lower, upper, size, number(2.0))

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

func Test_calculateArithmeticPins(t *testing.T) {
	type args struct {
		lower    fixedpoint.Value
		upper    fixedpoint.Value
		size     fixedpoint.Value
		tickSize fixedpoint.Value
	}
	tests := []struct {
		name string
		args args
		want []Pin
	}{
		{
			name: "simple",
			args: args{
				lower:    number(1000.0),
				upper:    number(3000.0),
				size:     number(30.0),
				tickSize: number(0.01),
			},
			want: []Pin{
				Pin(number(1000.0)),
				Pin(number(1066.660)),
				Pin(number(1133.330)),
				Pin(number(1199.990)),
				Pin(number(1266.660)),
				Pin(number(1333.330)),
				Pin(number(1399.990)),
				Pin(number(1466.660)),
				Pin(number(1533.330)),
				Pin(number(1599.990)),
				Pin(number(1666.660)),
				Pin(number(1733.330)),
				Pin(number(1799.990)),
				Pin(number(1866.660)),
				Pin(number(1933.330)),
				Pin(number(1999.990)),
				Pin(number(2066.660)),
				Pin(number(2133.330)),
				Pin(number("2199.99")),
				Pin(number(2266.660)),
				Pin(number(2333.330)),
				Pin(number("2399.99")),
				Pin(number(2466.660)),
				Pin(number(2533.330)),
				Pin(number("2599.99")),
				Pin(number(2666.660)),
				Pin(number(2733.330)),
				Pin(number(2799.990)),
				Pin(number(2866.660)),
				Pin(number(2933.330)),
				Pin(number(2999.990)),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, calculateArithmeticPins(tt.args.lower, tt.args.upper, tt.args.size, tt.args.tickSize), "calculateArithmeticPins(%v, %v, %v, %v)", tt.args.lower, tt.args.upper, tt.args.size, tt.args.tickSize)
		})
	}
}

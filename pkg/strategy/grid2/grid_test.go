package grid2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func number(a interface{}) fixedpoint.Value {
	switch v := a.(type) {
	case string:
		return fixedpoint.MustNewFromString(v)
	case int:
		return fixedpoint.NewFromInt(int64(v))
	case int64:
		return fixedpoint.NewFromInt(int64(v))
	case float64:
		return fixedpoint.NewFromFloat(v)
	}

	return fixedpoint.Zero
}

func TestNewGrid(t *testing.T) {
	upper := fixedpoint.NewFromFloat(500.0)
	lower := fixedpoint.NewFromFloat(100.0)
	size := fixedpoint.NewFromFloat(101.0)
	grid := NewGrid(lower, upper, size, number(0.01))
	grid.CalculateArithmeticPins()

	assert.Equal(t, upper, grid.UpperPrice)
	assert.Equal(t, lower, grid.LowerPrice)
	assert.Equal(t, fixedpoint.NewFromFloat(4), grid.Spread)
	if assert.Len(t, grid.Pins, 101) {
		assert.Equal(t, Pin(number(100.0)), grid.Pins[0])
		assert.Equal(t, Pin(number(500.0)), grid.Pins[100])
	}
}

func TestGrid_HasPin(t *testing.T) {
	upper := fixedpoint.NewFromFloat(500.0)
	lower := fixedpoint.NewFromFloat(100.0)
	size := fixedpoint.NewFromFloat(101.0)
	grid := NewGrid(lower, upper, size, number(0.01))
	grid.CalculateArithmeticPins()

	assert.True(t, grid.HasPin(Pin(number(100.0))))
	assert.True(t, grid.HasPin(Pin(number(500.0))))
	assert.False(t, grid.HasPin(Pin(number(101.0))))
}

func TestGrid_ExtendUpperPrice(t *testing.T) {
	upper := number(500.0)
	lower := number(100.0)
	size := number(5.0)
	grid := NewGrid(lower, upper, size, number(0.01))
	grid.CalculateArithmeticPins()

	originalSpread := grid.Spread

	t.Logf("pins: %+v", grid.Pins)
	assert.Equal(t, number(100.0), originalSpread)
	assert.Len(t, grid.Pins, 5)

	newPins := grid.ExtendUpperPrice(number(1000.0))
	assert.Len(t, grid.Pins, 10)
	assert.Len(t, newPins, 5)
	assert.Equal(t, originalSpread, grid.Spread)
	t.Logf("pins: %+v", grid.Pins)
}

func TestGrid_ExtendLowerPrice(t *testing.T) {
	upper := fixedpoint.NewFromFloat(3000.0)
	lower := fixedpoint.NewFromFloat(2000.0)
	size := fixedpoint.NewFromFloat(11.0)
	grid := NewGrid(lower, upper, size, number(0.01))
	grid.CalculateArithmeticPins()

	assert.Equal(t, Pin(number(2000.0)), grid.BottomPin(), "bottom pin should be 1000.0")
	assert.Equal(t, Pin(number(3000.0)), grid.TopPin(), "top pin should be 3000.0")
	assert.Len(t, grid.Pins, 11)

	// spread = (3000 - 2000) / 10.0
	expectedSpread := fixedpoint.NewFromFloat(100.0)
	assert.Equal(t, expectedSpread, grid.Spread)

	originalSpread := grid.Spread
	newPins := grid.ExtendLowerPrice(fixedpoint.NewFromFloat(1000.0))
	assert.Equal(t, originalSpread, grid.Spread)

	t.Logf("newPins: %+v", newPins)

	// 100 = (2000-1000) / 10
	if assert.Len(t, newPins, 10) {
		assert.Equal(t, Pin(number(1000.0)), newPins[0])
		assert.Equal(t, Pin(number(1900.0)), newPins[len(newPins)-1])
	}

	assert.Equal(t, expectedSpread, grid.Spread)

	if assert.Len(t, grid.Pins, 21) {
		assert.Equal(t, Pin(number(1000.0)), grid.BottomPin(), "bottom pin should be 1000.0")
		assert.Equal(t, Pin(number(3000.0)), grid.TopPin(), "top pin should be 3000.0")
	}
}

func TestGrid_NextLowerPin(t *testing.T) {
	upper := number(500.0)
	lower := number(100.0)
	size := number(5.0)
	grid := NewGrid(lower, upper, size, number(0.01))
	grid.CalculateArithmeticPins()

	t.Logf("pins: %+v", grid.Pins)

	next, ok := grid.NextLowerPin(number(200.0))
	assert.True(t, ok)
	assert.Equal(t, Pin(number(100.0)), next)

	next, ok = grid.NextLowerPin(number(150.0))
	assert.False(t, ok)
	assert.Equal(t, Pin(fixedpoint.Zero), next)
}

func TestGrid_NextHigherPin(t *testing.T) {
	upper := number(500.0)
	lower := number(100.0)
	size := number(5.0)
	grid := NewGrid(lower, upper, size, number(0.01))
	grid.CalculateArithmeticPins()
	t.Logf("pins: %+v", grid.Pins)

	next, ok := grid.NextHigherPin(number(100.0))
	assert.True(t, ok)
	assert.Equal(t, Pin(number(200.0)), next)

	next, ok = grid.NextHigherPin(number(400.0))
	assert.True(t, ok)
	assert.Equal(t, Pin(number(500.0)), next)

	next, ok = grid.NextHigherPin(number(500.0))
	assert.False(t, ok)
	assert.Equal(t, Pin(fixedpoint.Zero), next)
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
			// (3000-1000)/30 = 66.6666666
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
				Pin(number("1200.00")),
				Pin(number(1266.660)),
				Pin(number(1333.330)),
				Pin(number(1400.000)),
				Pin(number(1466.660)),
				Pin(number(1533.330)),
				Pin(number(1600.000)),
				Pin(number(1666.660)),
				Pin(number(1733.330)),
				Pin(number(1800.000)),
				Pin(number(1866.660)),
				Pin(number(1933.330)),
				Pin(number(2000.000)),
				Pin(number(2066.660)),
				Pin(number(2133.330)),
				Pin(number("2200.00")),
				Pin(number(2266.660)),
				Pin(number(2333.330)),
				Pin(number("2400.00")),
				Pin(number(2466.660)),
				Pin(number(2533.330)),
				Pin(number("2600.00")),
				Pin(number(2666.660)),
				Pin(number(2733.330)),
				Pin(number(2800.000)),
				Pin(number(2866.660)),
				Pin(number(2933.330)),
				Pin(number("3000.00")),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spread := tt.args.upper.Sub(tt.args.lower).Div(tt.args.size)
			pins := calculateArithmeticPins(tt.args.lower, tt.args.upper, spread, tt.args.tickSize)
			for i := 0; i < len(tt.want); i++ {
				assert.InDelta(t, fixedpoint.Value(tt.want[i]).Float64(),
					fixedpoint.Value(pins[i]).Float64(),
					0.001,
					"calculateArithmeticPins(%v, %v, %v, %v)", tt.args.lower, tt.args.upper, tt.args.size, tt.args.tickSize)
			}
		})
	}
}

func Test_filterPrice1(t *testing.T) {
	type args struct {
		p    fixedpoint.Value
		prec int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "basic",
			args: args{p: number("31.2222"), prec: 3},
			want: "31.222",
		},
		{
			name: "roundup",
			args: args{p: number("31.22295"), prec: 3},
			want: "31.223",
		},
		{
			name: "roundup2",
			args: args{p: number("31.22290"), prec: 3},
			want: "31.222",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rst := filterPrice(tt.args.p, tt.args.prec)
			assert.Equalf(t, tt.want, rst.String(), "filterPrice(%v, %v)", tt.args.p, tt.args.prec)
		})
	}
}

func Test_removeDuplicatedPins(t *testing.T) {
	pins := []Pin{
		Pin(number("31.222")),
		Pin(number("31.222")),
		Pin(number("31.223")),
		Pin(number("31.224")),
		Pin(number("31.224")),
	}
	out := removeDuplicatedPins(pins)
	assert.Equal(t, []Pin{
		Pin(number("31.222")),
		Pin(number("31.223")),
		Pin(number("31.224")),
	}, out)

}

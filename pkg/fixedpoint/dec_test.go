package fixedpoint

import (
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
)

const Delta = 1e-9

func BenchmarkMul(b *testing.B) {
	b.ResetTimer()

	b.Run("mul-float64", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			x := NewFromFloat(20.0)
			y := NewFromFloat(20.0)
			x = x.Mul(y)
		}
	})

	b.Run("mul-float64-large-numbers", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			x := NewFromFloat(88.12345678)
			y := NewFromFloat(88.12345678)
			x = x.Mul(y)
		}
	})

	b.Run("mul-big-small-numbers", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			x := big.NewFloat(20.0)
			y := big.NewFloat(20.0)
			x = new(big.Float).Mul(x, y)
		}
	})

	b.Run("mul-big-large-numbers", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			x := big.NewFloat(88.12345678)
			y := big.NewFloat(88.12345678)
			x = new(big.Float).Mul(x, y)
		}
	})
}

func TestMulString(t *testing.T) {
	x := NewFromFloat(10.55)
	assert.Equal(t, "10.55", x.String())
	y := NewFromFloat(10.55)
	x = x.Mul(y)
	assert.Equal(t, "111.3025", x.String())
	assert.Equal(t, "111.30", x.FormatString(2))
	assert.InDelta(t, 111.3025, x.Float64(), Delta)
}

func TestMulExp(t *testing.T) {
	x, _ := NewFromString("166")
	digits := x.NumIntDigits()
	assert.Equal(t, digits, 3)
	step := x.MulExp(-digits + 1)
	assert.Equal(t, "1.66", step.String())
}

func TestNew(t *testing.T) {
	f := NewFromFloat(0.001)
	assert.Equal(t, "0.001", f.String())
	assert.Equal(t, "0.0010", f.FormatString(4))
	assert.Equal(t, "0.1%", f.Percentage())
	assert.Equal(t, "0.10%", f.FormatPercentage(2))
}

func TestRound(t *testing.T) {
	f := NewFromFloat(1.2345)
	f = f.Round(0, Down)
	assert.Equal(t, "1", f.String())
	w := NewFromFloat(1.2345)
	w = w.Trunc()
	assert.Equal(t, "1", w.String())
	s := NewFromFloat(1.2345)
	assert.Equal(t, "1.23", s.Round(2, Down).String())
}

func TestFromString(t *testing.T) {
	f := MustNewFromString("0.004075")
	assert.Equal(t, "0.004075", f.String())
	f = MustNewFromString("0.03")
	assert.Equal(t, "0.03", f.String())

	f = MustNewFromString("0.75%")
	assert.Equal(t, "0.0075", f.String())
	f = MustNewFromString("1.1e-7")
	assert.Equal(t, "0.00000011", f.String())
	f = MustNewFromString(".0%")
	assert.Equal(t, Zero, f)
}

func TestNumFractionalDigits(t *testing.T) {
	tests := []struct {
		name string
		v    Value
		want int
	}{
		{
			name: "ignore the integer part",
			v:    MustNewFromString("123.4567"),
			want: 4,
		},
		{
			name: "ignore the sign",
			v:    MustNewFromString("-123.4567"),
			want: 4,
		},
		{
			name: "ignore the trailing zero",
			v:    MustNewFromString("-123.45000000"),
			want: 2,
		},
		{
			name: "no fractional parts",
			v:    MustNewFromString("-1"),
			want: 0,
		},
		{
			name: "no fractional parts",
			v:    MustNewFromString("-1.0"),
			want: 0,
		},
		{
			name: "only fractional part",
			v:    MustNewFromString(".123456"),
			want: 6,
		},
		{
			name: "percentage",
			v:    MustNewFromString("0.075%"), // 0.075 * 0.01
			want: 5,
		},
		{
			name: "scientific notation",
			v:    MustNewFromString("1.1e-7"),
			want: 8,
		},
		{
			name: "zero",
			v:    MustNewFromString("0"),
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.NumFractionalDigits(); got != tt.want {
				t.Errorf("NumFractionalDigits() = %v, want %v", got, tt.want)
			}
		})
	}
}

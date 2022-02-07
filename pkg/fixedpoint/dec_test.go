package fixedpoint

import (
	"testing"
    "math/big"
	"github.com/stretchr/testify/assert"
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
	step := x.MulExp(-digits+1)
	assert.Equal(t, "1.66", step.String())
}

func TestNew(t *testing.T) {
	f := NewFromFloat(0.001)
	assert.Equal(t, "0.001", f.String())
	assert.Equal(t, "0.0010", f.FormatString(4))
	assert.Equal(t, "0.1%", f.Percentage())
	assert.Equal(t, "0.10%", f.FormatPercentage(2))
}

// Not used
/*func TestParse(t *testing.T) {
	type args struct {
		input string
	}
	tests := []struct {
		name                 string
		args                 args
		wantNum              int64
		wantNumDecimalPoints int
		wantErr              bool
	}{
		{
			args:                 args{input: "-99.9"},
			wantNum:              -999,
			wantNumDecimalPoints: 1,
			wantErr:              false,
		},
		{
			args:                 args{input: "0.75%"},
			wantNum:              75,
			wantNumDecimalPoints: 4,
			wantErr:              false,
		},
		{
			args:                 args{input: "0.12345678"},
			wantNum:              12345678,
			wantNumDecimalPoints: 8,
			wantErr:              false,
		},
		{
			args:                 args{input: "a"},
			wantNum:              0,
			wantNumDecimalPoints: 0,
			wantErr:              true,
		},
		{
			args:                 args{input: "0.1"},
			wantNum:              1,
			wantNumDecimalPoints: 1,
			wantErr:              false,
		},
		{
			args:                 args{input: "100"},
			wantNum:              100,
			wantNumDecimalPoints: 0,
			wantErr:              false,
		},
		{
			args:                 args{input: "100.9999"},
			wantNum:              1009999,
			wantNumDecimalPoints: 4,
			wantErr:              false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNum, gotNumDecimalPoints, err := Parse(tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotNum != tt.wantNum {
				t.Errorf("Parse() gotNum = %v, want %v", gotNum, tt.wantNum)
			}
			if gotNumDecimalPoints != tt.wantNumDecimalPoints {
				t.Errorf("Parse() gotNumDecimalPoints = %v, want %v", gotNumDecimalPoints, tt.wantNumDecimalPoints)
			}
		})
	}
}*/

func TestNumFractionalDigits(t *testing.T) {
	tests := []struct {
		name string
		v    Value
		want int
	}{
		{
			name: "over the default precision",
			v:    MustNewFromString("0.123456789"),
			want: 9,
		},
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.NumFractionalDigits(); got != tt.want {
				t.Errorf("NumFractionalDigits() = %v, want %v", got, tt.want)
			}
		})
	}
}

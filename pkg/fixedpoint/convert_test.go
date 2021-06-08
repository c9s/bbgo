package fixedpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
			x := NewFromFloat(20.0)
			y := NewFromFloat(20.0)
			x = x.BigMul(y)
		}
	})

	b.Run("mul-big-large-numbers", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			x := NewFromFloat(88.12345678)
			y := NewFromFloat(88.12345678)
			x = x.BigMul(y)
		}
	})
}

func TestBigMul(t *testing.T) {
	x := NewFromFloat(10.55)
	y := NewFromFloat(10.55)
	x = x.BigMul(y)
	assert.Equal(t, NewFromFloat(111.3025), x)
}

func TestParse(t *testing.T) {
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
}

func TestNumFractionalDigits(t *testing.T) {
	tests := []struct {
		name string
		v    Value
		want int
	}{
		{
			name: "over the default precision",
			v:    MustNewFromString("0.123456789"),
			want: 8,
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
			if got := NumFractionalDigits(tt.v); got != tt.want {
				t.Errorf("NumFractionalDigits() = %v, want %v", got, tt.want)
			}
		})
	}
}

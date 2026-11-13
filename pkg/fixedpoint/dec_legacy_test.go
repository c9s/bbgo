//go:build !dnum

package fixedpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNumFractionalDigitsLegacy(t *testing.T) {
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
			name: "zero underflow",
			v:    MustNewFromString("1e-100"),
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.NumFractionalDigits(); got != tt.want {
				t.Errorf("NumFractionalDigitsLegacy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_Overflow(t *testing.T) {
	t.Run("NewFromString overflow", func(t *testing.T) {
		large := "999999999999999999.99"
		v, err := NewFromString(large)
		assert.NoError(t, err)
		assert.Equal(t, PosInf, v)
		assert.True(t, v.IsInf())

		largeNeg := "-999999999999999999.99"
		vNeg, err := NewFromString(largeNeg)
		assert.NoError(t, err)
		assert.Equal(t, NegInf, vNeg)
		assert.True(t, vNeg.IsInf())
	})

	t.Run("Add overflow", func(t *testing.T) {
		a := PosInf.Sub(NewFromInt(100))
		b := NewFromInt(200)
		res := a.Add(b)
		assert.Equal(t, PosInf, res)
	})

	t.Run("Sub underflow", func(t *testing.T) {
		a := NegInf.Add(NewFromInt(100))
		b := NewFromInt(200)
		res := a.Sub(b)
		assert.Equal(t, NegInf, res)
	})
}


//go:build !dnum

package fixedpoint

import (
	"testing"
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

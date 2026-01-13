//go:build !dnum
// +build !dnum

package xmaker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

func Test_aggregatePrice(t *testing.T) {
	bids := PriceVolumeSliceFromText(`
	1000.0, 1.0
	1200.0, 1.0
	1400.0, 1.0
`)

	aggregatedPrice1 := aggregatePrice(bids, fixedpoint.NewFromFloat(0.5))
	assert.Equal(t, fixedpoint.NewFromFloat(1000.0), aggregatedPrice1)

	aggregatedPrice2 := aggregatePrice(bids, fixedpoint.NewFromInt(1))
	assert.Equal(t, fixedpoint.NewFromFloat(1000.0), aggregatedPrice2)

	aggregatedPrice3 := aggregatePrice(bids, fixedpoint.NewFromInt(2))
	assert.Equal(t, fixedpoint.NewFromFloat(1100.0), aggregatedPrice3)
}

func Test_calculateSpread(t *testing.T) {
	tests := []struct {
		name string
		a    fixedpoint.Value
		b    fixedpoint.Value
		want fixedpoint.Value
	}{
		{
			name: "positive spread",
			a:    fixedpoint.NewFromFloat(100.0),
			b:    fixedpoint.NewFromFloat(90.0),
			want: fixedpoint.NewFromFloat(0.1), // (100 - 90) / 100 = 0.1
		},
		{
			name: "negative spread",
			a:    fixedpoint.NewFromFloat(100.0),
			b:    fixedpoint.NewFromFloat(110.0),
			want: fixedpoint.NewFromFloat(-0.1), // (100 - 110) / 100 = -0.1
		},
		{
			name: "zero spread",
			a:    fixedpoint.NewFromFloat(100.0),
			b:    fixedpoint.NewFromFloat(100.0),
			want: fixedpoint.Zero,
		},
		{
			name: "zero divisor",
			a:    fixedpoint.Zero,
			b:    fixedpoint.NewFromFloat(100.0),
			want: fixedpoint.Zero,
		},
		{
			name: "both zero",
			a:    fixedpoint.Zero,
			b:    fixedpoint.Zero,
			want: fixedpoint.Zero,
		},
		{
			name: "negative a",
			a:    fixedpoint.NewFromFloat(-100.0),
			b:    fixedpoint.NewFromFloat(-90.0),
			want: fixedpoint.NewFromFloat(0.1), // (-100 - (-90)) / -100 = -10 / -100 = 0.1
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateSpread(tt.a, tt.b)
			assert.Equal(t, tt.want, got)
		})
	}
}

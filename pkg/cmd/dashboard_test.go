package cmd

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	testifyassert "github.com/stretchr/testify/assert"
)

func TestAggregatePrice(t *testing.T) {
	price := fixedpoint.MustNewFromString("63773.91")

	tests := []struct {
		name string
		unit fixedpoint.Value
		want fixedpoint.Value
	}{
		{
			name: "zero disables aggregation",
			unit: fixedpoint.Zero,
			want: fixedpoint.MustNewFromString("63773.91"),
		},
		{
			name: "negative disables aggregation",
			unit: fixedpoint.MustNewFromString("-1"),
			want: fixedpoint.MustNewFromString("63773.91"),
		},
		{
			name: "one tenth",
			unit: fixedpoint.MustNewFromString("0.1"),
			want: fixedpoint.MustNewFromString("63773.9"),
		},
		{
			name: "cent",
			unit: fixedpoint.MustNewFromString("0.01"),
			want: fixedpoint.MustNewFromString("63773.91"),
		},
		{
			name: "one",
			unit: fixedpoint.One,
			want: fixedpoint.MustNewFromString("63773"),
		},
		{
			name: "five",
			unit: fixedpoint.NewFromInt(5),
			want: fixedpoint.MustNewFromString("63770"),
		},
		{
			name: "ten",
			unit: fixedpoint.NewFromInt(10),
			want: fixedpoint.MustNewFromString("63770"),
		},
		{
			name: "larger fractional step",
			unit: fixedpoint.MustNewFromString("2.5"),
			want: fixedpoint.MustNewFromString("63772.5"),
		},
		{
			name: "sub-unit floors down, not to nearest",
			unit: fixedpoint.MustNewFromString("0.5"),
			want: fixedpoint.MustNewFromString("63773.5"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testifyassert.Equal(t, tt.want, aggregatePrice(price, tt.unit))
		})
	}
}

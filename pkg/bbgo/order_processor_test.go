package bbgo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestAdjustQuantityByMinAmount(t *testing.T) {
	type args struct {
		quantity, price, minAmount fixedpoint.Value
	}
	type testcase struct {
		name   string
		args   args
		wanted string
	}

	tests := []testcase{
		{
			name:   "amount too small",
			args:   args{
				fixedpoint.MustNewFromString("0.1"),
				fixedpoint.MustNewFromString("10.0"),
				fixedpoint.MustNewFromString("10.0"),
			},
			wanted: "1.0",
		},
		{
			name:   "amount equals to min amount",
			args:   args{
				fixedpoint.MustNewFromString("1.0"),
				fixedpoint.MustNewFromString("10.0"),
				fixedpoint.MustNewFromString("10.0"),
			},
			wanted: "1.0",
		},
		{
			name:   "amount is greater than min amount",
			args:   args{
				fixedpoint.MustNewFromString("2.0"),
				fixedpoint.MustNewFromString("10.0"),
				fixedpoint.MustNewFromString("10.0"),
			},
			wanted: "2.0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q := AdjustFloatQuantityByMinAmount(test.args.quantity, test.args.price, test.args.minAmount)
			assert.Equal(t, test.wanted, q.String())
		})
	}
}

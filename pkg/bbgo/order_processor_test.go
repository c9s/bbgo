package bbgo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdjustQuantityByMinAmount(t *testing.T) {
	type args struct {
		quantity, price, minAmount float64
	}
	type testcase struct {
		name   string
		args   args
		wanted float64
	}

	tests := []testcase{
		{
			name:   "amount too small",
			args:   args{0.1, 10.0, 10.0},
			wanted: 1.0,
		},
		{
			name:   "amount equals to min amount",
			args:   args{1.0, 10.0, 10.0},
			wanted: 1.0,
		},
		{
			name:   "amount is greater than min amount",
			args:   args{2.0, 10.0, 10.0},
			wanted: 2.0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q := AdjustQuantityByMinAmount(test.args.quantity, test.args.price, test.args.minAmount)
			assert.Equal(t, test.wanted, q)
		})
	}
}

package xgap

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/stretchr/testify/assert"
)

func Test_AdjustPrice(t *testing.T) {
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
			args: args{p: fixedpoint.MustNewFromString("0.32179999"), prec: 4},
			want: "0.3218",
		},
		{
			name: "adjust",
			args: args{p: fixedpoint.MustNewFromString("0.32148"), prec: 4},
			want: "0.3214",
		},
		{
			name: "adjust2",
			args: args{p: fixedpoint.MustNewFromString("0.321600"), prec: 4},
			want: "0.3216",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rst := adjustPrice(tt.args.p, tt.args.prec)
			assert.Equalf(t, tt.want, rst.String(), "AdjustPrice(%v, %v)", tt.args.p, tt.args.prec)
		})
	}
}

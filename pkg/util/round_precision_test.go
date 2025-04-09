package util

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/stretchr/testify/assert"
)

func number(a interface{}) fixedpoint.Value {
	switch v := a.(type) {
	case string:
		return fixedpoint.MustNewFromString(v)
	case int:
		return fixedpoint.NewFromInt(int64(v))
	case int64:
		return fixedpoint.NewFromInt(int64(v))
	case float64:
		return fixedpoint.NewFromFloat(v)
	}

	return fixedpoint.Zero
}

func Test_RoundPrice(t *testing.T) {
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
			args: args{p: number("31.2222"), prec: 3},
			want: "31.222",
		},
		{
			name: "roundup",
			args: args{p: number("31.22295"), prec: 3},
			want: "31.223",
		},
		{
			name: "roundup2",
			args: args{p: number("31.22290"), prec: 3},
			want: "31.222",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rst := RoundAndTruncatePrice(tt.args.p, tt.args.prec)
			assert.Equalf(t, tt.want, rst.String(), "RoundAndTruncatePrice(%v, %v)", tt.args.p, tt.args.prec)
		})
	}
}

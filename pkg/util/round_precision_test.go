package util

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/stretchr/testify/assert"
)

func Test_RoundAndTruncatePrice(t *testing.T) {
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
			args: args{p: testhelper.Number("31.2222"), prec: 3},
			want: "31.222",
		},
		{
			name: "roundup",
			args: args{p: testhelper.Number("31.22295"), prec: 3},
			want: "31.223",
		},
		{
			name: "roundup2",
			args: args{p: testhelper.Number("31.22290"), prec: 3},
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

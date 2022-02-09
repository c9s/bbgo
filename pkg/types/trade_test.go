package types

import "testing"
import "github.com/c9s/bbgo/pkg/fixedpoint"

func Test_trimTrailingZero(t *testing.T) {
	type args struct {
		a string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "trailing floating zero",
			args: args{
				a: "1.23400000",
			},
			want: "1.234",
		},
		{
			name: "trailing zero of an integer",
			args: args{
				a: "1.00000",
			},
			want: "1",
		},
		{
			name: "non trailing zero",
			args: args{
				a: "1.0001234567",
			},
			want: "1.0001234567",
		},
		{
			name: "integer",
			args: args{
				a: "1200000",
			},
			want: "1200000",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fixedpoint.MustNewFromString(tt.args.a).String(); got != tt.want {
				t.Errorf("trimTrailingZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

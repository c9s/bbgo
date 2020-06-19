package bbgo

import "testing"

func TestNotZero(t *testing.T) {
	type args struct {
		v float64
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "0",
			args: args{
				v: 0,
			},
			want: false,
		},
		{
			name: "0.00001",
			args: args{
				v: 0.00001,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NotZero(tt.args.v); got != tt.want {
				t.Errorf("NotZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

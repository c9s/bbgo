package linregmaker

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func Test_calculateBandPercentage(t *testing.T) {
	type args struct {
		up       float64
		down     float64
		sma      float64
		midPrice float64
	}
	tests := []struct {
		name string
		args args
		want fixedpoint.Value
	}{
		{
			name: "positive boundary",
			args: args{
				up:       2000.0,
				sma:      1500.0,
				down:     1000.0,
				midPrice: 2000.0,
			},
			want: fixedpoint.NewFromFloat(1.0),
		},
		{
			name: "inside positive boundary",
			args: args{
				up:       2000.0,
				sma:      1500.0,
				down:     1000.0,
				midPrice: 1600.0,
			},
			want: fixedpoint.NewFromFloat(0.2), // 20%
		},
		{
			name: "negative boundary",
			args: args{
				up:       2000.0,
				sma:      1500.0,
				down:     1000.0,
				midPrice: 1000.0,
			},
			want: fixedpoint.NewFromFloat(-1.0),
		},
		{
			name: "out of negative boundary",
			args: args{
				up:       2000.0,
				sma:      1500.0,
				down:     1000.0,
				midPrice: 800.0,
			},
			want: fixedpoint.NewFromFloat(-1.4),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calculateBandPercentage(tt.args.up, tt.args.down, tt.args.sma, tt.args.midPrice); fixedpoint.NewFromFloat(got) != tt.want {
				t.Errorf("calculateBandPercentage() = %v, want %v", got, tt.want)
			}
		})
	}
}

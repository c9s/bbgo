package risk

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestCalculateMarginCost(t *testing.T) {
	type args struct {
		price    fixedpoint.Value
		quantity fixedpoint.Value
		leverage fixedpoint.Value
	}
	tests := []struct {
		name string
		args args
		want fixedpoint.Value
	}{
		{
			name: "simple",
			args: args{
				price:    fixedpoint.NewFromFloat(9000.0),
				quantity: fixedpoint.NewFromFloat(2.0),
				leverage: fixedpoint.NewFromFloat(3.0),
			},
			want: fixedpoint.NewFromFloat(9000.0 * 2.0 / 3.0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateMarginCost(tt.args.price, tt.args.quantity, tt.args.leverage); got.String() != tt.want.String() {
				t.Errorf("CalculateMarginCost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculatePositionCost(t *testing.T) {
	type args struct {
		markPrice  fixedpoint.Value
		orderPrice fixedpoint.Value
		quantity   fixedpoint.Value
		leverage   fixedpoint.Value
		side       types.SideType
	}
	tests := []struct {
		name string
		args args
		want fixedpoint.Value
	}{
		{
			// long position does not have openLoss
			name: "long",
			args: args{
				markPrice:  fixedpoint.NewFromFloat(9050.0),
				orderPrice: fixedpoint.NewFromFloat(9000.0),
				quantity:   fixedpoint.NewFromFloat(2.0),
				leverage:   fixedpoint.NewFromFloat(3.0),
				side:       types.SideTypeBuy,
			},
			want: fixedpoint.NewFromFloat(6000.0),
		},
		{
			// long position does not have openLoss
			name: "short",
			args: args{
				markPrice:  fixedpoint.NewFromFloat(9050.0),
				orderPrice: fixedpoint.NewFromFloat(9000.0),
				quantity:   fixedpoint.NewFromFloat(2.0),
				leverage:   fixedpoint.NewFromFloat(3.0),
				side:       types.SideTypeSell,
			},
			want: fixedpoint.NewFromFloat(6100.0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculatePositionCost(tt.args.markPrice, tt.args.orderPrice, tt.args.quantity, tt.args.leverage, tt.args.side); got.String() != tt.want.String() {
				t.Errorf("CalculatePositionCost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateMaxPosition(t *testing.T) {
	type args struct {
		price           fixedpoint.Value
		availableMargin fixedpoint.Value
		leverage        fixedpoint.Value
	}
	tests := []struct {
		name string
		args args
		want fixedpoint.Value
	}{
		{
			name: "3x",
			args: args{
				price:           fixedpoint.NewFromFloat(9000.0),
				availableMargin: fixedpoint.NewFromFloat(300.0),
				leverage:        fixedpoint.NewFromFloat(3.0),
			},
			want: fixedpoint.NewFromFloat(0.1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateMaxPosition(tt.args.price, tt.args.availableMargin, tt.args.leverage); got.String() != tt.want.String() {
				t.Errorf("CalculateMaxPosition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateMinRequiredLeverage(t *testing.T) {
	type args struct {
		price           fixedpoint.Value
		quantity        fixedpoint.Value
		availableMargin fixedpoint.Value
	}
	tests := []struct {
		name string
		args args
		want fixedpoint.Value
	}{
		{
			name: "30x",
			args: args{
				price:           fixedpoint.NewFromFloat(9000.0),
				quantity:        fixedpoint.NewFromFloat(10.0),
				availableMargin: fixedpoint.NewFromFloat(3000.0),
			},
			want: fixedpoint.NewFromFloat(30.0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateMinRequiredLeverage(tt.args.price, tt.args.quantity, tt.args.availableMargin); got.String() != tt.want.String() {
				t.Errorf("CalculateMinRequiredLeverage() = %v, want %v", got, tt.want)
			}
		})
	}
}

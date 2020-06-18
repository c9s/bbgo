package bbgo

import (
	"github.com/adshao/go-binance"
	"testing"
)

func TestVolumeByPriceChange(t *testing.T) {
	type args struct {
		market       Market
		currentPrice float64
		change       float64
		side         binance.SideType
	}
	tests := []struct {
		name string
		args args
		want float64
	}{
		{
			name: "buy-change-50-at-9400",
			args: args{
				market: MarketBTCUSDT,
				currentPrice: 9400,
				change: 50,
				side: binance.SideTypeBuy,
			},
			want: 0.00106382,
		},
		{
			name: "buy-change-100-at-9200",
			args: args{
				market: MarketBTCUSDT,
				currentPrice: 9200,
				change: 100,
				side: binance.SideTypeBuy,
			},
			want: 0.00108695,
		},
		{
			name: "sell-change-100-at-9500",
			args: args{
				market: MarketBTCUSDT,
				currentPrice: 9500,
				change: 100,
				side: binance.SideTypeSell,
			},
			want: 0.00249052,
		},
		{
			name: "sell-change-200-at-9600",
			args: args{
				market: MarketBTCUSDT,
				currentPrice: 9500,
				change: 200,
				side: binance.SideTypeSell,
			},
			want: 0.00265114,
		},
		{
			name: "sell-change-500-at-9600",
			args: args{
				market: MarketBTCUSDT,
				currentPrice: 9600,
				change: 500,
				side: binance.SideTypeSell,
			},
			want: 0.00390591,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := VolumeByPriceChange(tt.args.market, tt.args.currentPrice, tt.args.change, tt.args.side); got != tt.want {
				t.Errorf("VolumeByPriceChange() = %v, want %v", got, tt.want)
			}
		})
	}
}

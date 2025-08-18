package xgap

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
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
		{
			name: "negPrecision",
			args: args{p: fixedpoint.MustNewFromString("0.3213333"), prec: -1},
			want: "0.3213333",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rst := adjustPrice(tt.args.p, tt.args.prec)
			assert.Equalf(t, tt.want, rst.String(), "adjustPrice(%v, %v)", tt.args.p, tt.args.prec)
		})
	}
}

func Test_adjustPositionOrder(t *testing.T) {
	tradingMarket := types.Market{
		Exchange:        "NotRealExchange",
		Symbol:          "NOTREAL",
		BaseCurrency:    "NOT",
		QuoteCurrency:   "REAL",
		PricePrecision:  4,
		VolumePrecision: 4,
	}
	type args struct {
		bestBid, bestAsk types.PriceVolume
		currentPosition  *types.Position
	}
	tests := []struct {
		name               string
		args               args
		expectAvailability bool
		expectOrderPrice   fixedpoint.Value
		expectSide         types.SideType
	}{
		{
			name: "Short_Position",
			args: args{
				bestBid: types.PriceVolume{
					Price:  fixedpoint.MustNewFromString("0.3218"),
					Volume: fixedpoint.MustNewFromString("0.1"),
				},
				bestAsk: types.PriceVolume{
					Price:  fixedpoint.MustNewFromString("0.3219"),
					Volume: fixedpoint.MustNewFromString("0.1"),
				},
				currentPosition: &types.Position{
					BaseCurrency:  tradingMarket.BaseCurrency,
					QuoteCurrency: tradingMarket.QuoteCurrency,
					Market:        tradingMarket,
					Base:          fixedpoint.NewFromFloat(-123),
					AverageCost:   fixedpoint.MustNewFromString("0.3220"),
				},
			},
			expectAvailability: true,
			expectOrderPrice:   fixedpoint.MustNewFromString("0.3219"),
			expectSide:         types.SideTypeBuy,
		},
		{
			name: "Long_Position",
			args: args{
				bestBid: types.PriceVolume{
					Price:  fixedpoint.MustNewFromString("0.3218"),
					Volume: fixedpoint.MustNewFromString("0.1"),
				},
				bestAsk: types.PriceVolume{
					Price:  fixedpoint.MustNewFromString("0.3219"),
					Volume: fixedpoint.MustNewFromString("0.1"),
				},
				currentPosition: &types.Position{
					BaseCurrency:  tradingMarket.BaseCurrency,
					QuoteCurrency: tradingMarket.QuoteCurrency,
					Market:        tradingMarket,
					Base:          fixedpoint.NewFromFloat(123),
					AverageCost:   fixedpoint.MustNewFromString("0.3215"),
				},
			},
			expectAvailability: true,
			expectOrderPrice:   fixedpoint.MustNewFromString("0.3218"),
			expectSide:         types.SideTypeSell,
		},
		{
			name: "Zero_Position",
			args: args{
				bestBid: types.PriceVolume{
					Price:  fixedpoint.MustNewFromString("0.3218"),
					Volume: fixedpoint.MustNewFromString("0.1"),
				},
				bestAsk: types.PriceVolume{
					Price:  fixedpoint.MustNewFromString("0.3219"),
					Volume: fixedpoint.MustNewFromString("0.1"),
				},
				currentPosition: &types.Position{
					BaseCurrency:  tradingMarket.BaseCurrency,
					QuoteCurrency: tradingMarket.QuoteCurrency,
					Market:        tradingMarket,
					Base:          fixedpoint.Zero,
					AverageCost:   fixedpoint.MustNewFromString("0.32185"),
				},
			},
			expectAvailability: false,
		},
		{
			name: "Short_Position_No_Adjustment",
			args: args{
				bestBid: types.PriceVolume{
					Price:  fixedpoint.MustNewFromString("0.3218"),
					Volume: fixedpoint.MustNewFromString("0.1"),
				},
				bestAsk: types.PriceVolume{
					Price:  fixedpoint.MustNewFromString("0.3219"),
					Volume: fixedpoint.MustNewFromString("0.1"),
				},
				currentPosition: &types.Position{
					BaseCurrency:  tradingMarket.BaseCurrency,
					QuoteCurrency: tradingMarket.QuoteCurrency,
					Market:        tradingMarket,
					Base:          fixedpoint.NewFromFloat(-123),
					AverageCost:   fixedpoint.MustNewFromString("0.32185"),
				},
			},
			expectAvailability: false,
		},
		{
			name: "Long_Position_No_Adjustment",
			args: args{
				bestBid: types.PriceVolume{
					Price:  fixedpoint.MustNewFromString("0.3218"),
					Volume: fixedpoint.MustNewFromString("0.1"),
				},
				bestAsk: types.PriceVolume{
					Price:  fixedpoint.MustNewFromString("0.3219"),
					Volume: fixedpoint.MustNewFromString("0.1"),
				},
				currentPosition: &types.Position{
					BaseCurrency:  tradingMarket.BaseCurrency,
					QuoteCurrency: tradingMarket.QuoteCurrency,
					Market:        tradingMarket,
					Base:          fixedpoint.NewFromFloat(123),
					AverageCost:   fixedpoint.MustNewFromString("0.32185"),
				},
			},
			expectAvailability: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, adjPosOrder := buildAdjustPositionOrder(
				tradingMarket.Symbol,
				tt.args.currentPosition,
				tt.args.bestBid,
				tt.args.bestAsk)
			assert.Equal(t, tt.expectAvailability, ok)
			if tt.expectAvailability {
				assert.Equal(t, types.TimeInForceIOC, adjPosOrder.TimeInForce)
				assert.Equal(t, adjPosOrder.Price.String(), tt.expectOrderPrice.String())
				assert.Equal(t, adjPosOrder.Side, tt.expectSide)
			}
		})
	}
}

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
		name             string
		args             args
		expectNumOrders  int
		expectOrderPrice fixedpoint.Value
		expectSide       types.SideType
	}{
		{
			name: "Short Position",
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
			expectNumOrders:  1,
			expectOrderPrice: fixedpoint.MustNewFromString("0.3219"),
			expectSide:       types.SideTypeBuy,
		},
		{
			name: "Long Position",
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
			expectNumOrders:  1,
			expectOrderPrice: fixedpoint.MustNewFromString("0.3218"),
			expectSide:       types.SideTypeSell,
		},
		{
			name: "Zero Position",
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
			expectNumOrders: 0,
		},
		{
			name: "Short Position - No Adjustment",
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
			expectNumOrders: 0,
		},
		{
			name: "Long Position - No Adjustment",
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
			expectNumOrders: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orders := getAdjustPositionOrder(tradingMarket.Symbol, tradingMarket, tt.args.currentPosition, tt.args.bestBid, tt.args.bestAsk)
			assert.Equal(t, len(orders), tt.expectNumOrders)
			if tt.expectNumOrders > 0 {
				order := orders[0]
				assert.Equal(t, order.Price.String(), tt.expectOrderPrice.String())
				assert.Equal(t, order.Side, tt.expectSide)
			}
		})
	}
}

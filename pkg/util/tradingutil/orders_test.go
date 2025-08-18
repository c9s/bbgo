package tradingutil

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_HasSufficientBalance(t *testing.T) {
	tradingMarket := types.Market{
		Exchange:        "NotRealExchange",
		Symbol:          "NOTREAL",
		BaseCurrency:    "NOT",
		QuoteCurrency:   "REAL",
		PricePrecision:  4,
		VolumePrecision: 4,
	}
	type args struct {
		submitOrder types.SubmitOrder
		balances    types.BalanceMap
	}
	tests := []struct {
		name               string
		args               args
		expectAvailability bool
	}{
		{
			name: "buy",
			args: args{
				submitOrder: types.SubmitOrder{
					Side:     types.SideTypeBuy,
					Quantity: fixedpoint.NewFromFloat(1),
					Price:    fixedpoint.NewFromFloat(100),
				},
				balances: types.BalanceMap{
					tradingMarket.QuoteCurrency: {
						Available: fixedpoint.NewFromFloat(100),
					},
				},
			},
			expectAvailability: true,
		},
		{
			name: "sell",
			args: args{
				submitOrder: types.SubmitOrder{
					Side:     types.SideTypeSell,
					Quantity: fixedpoint.NewFromFloat(1),
					Price:    fixedpoint.NewFromFloat(100),
				},
				balances: types.BalanceMap{
					tradingMarket.BaseCurrency: {
						Available: fixedpoint.NewFromFloat(1),
					},
				},
			},
			expectAvailability: true,
		},
		{
			name: "buy - insufficient balance",
			args: args{
				submitOrder: types.SubmitOrder{
					Side:     types.SideTypeBuy,
					Quantity: fixedpoint.NewFromFloat(1),
					Price:    fixedpoint.NewFromFloat(100),
				},
				balances: types.BalanceMap{
					tradingMarket.QuoteCurrency: {
						Available: fixedpoint.NewFromFloat(10),
					},
				},
			},
			expectAvailability: false,
		},
		{
			name: "sell - insufficient balance",
			args: args{
				submitOrder: types.SubmitOrder{
					Side:     types.SideTypeSell,
					Quantity: fixedpoint.NewFromFloat(10),
					Price:    fixedpoint.NewFromFloat(100),
				},
				balances: types.BalanceMap{
					tradingMarket.BaseCurrency: {
						Available: fixedpoint.NewFromFloat(1),
					},
				},
			},
			expectAvailability: false,
		},
		{
			name: "buy - invalid currency",
			args: args{
				submitOrder: types.SubmitOrder{
					Side:     types.SideTypeBuy,
					Quantity: fixedpoint.NewFromFloat(1),
					Price:    fixedpoint.NewFromFloat(1),
				},
				balances: types.BalanceMap{
					"HelloCoin": {
						Available: fixedpoint.NewFromFloat(100),
					},
				},
			},
			expectAvailability: false,
		},
		{
			name: "sell - invalid currency",
			args: args{
				submitOrder: types.SubmitOrder{
					Side:     types.SideTypeSell,
					Quantity: fixedpoint.NewFromFloat(1),
					Price:    fixedpoint.NewFromFloat(1),
				},
				balances: types.BalanceMap{
					"HelloCoin": {
						Available: fixedpoint.NewFromFloat(100),
					},
				},
			},
			expectAvailability: false,
		},
	}
	for _, testCase := range tests {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				isAvailable, err := HasSufficientBalance(
					tradingMarket,
					testCase.args.submitOrder,
					testCase.args.balances,
				)
				assert.Equal(t, testCase.expectAvailability, isAvailable)
				if isAvailable {
					assert.NoError(t, err)
				} else {
					assert.Error(t, err)
				}
			},
		)
	}
}

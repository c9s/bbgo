package riskcontrol

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_BalanceCheck(t *testing.T) {
	cases := []struct {
		Currency               string
		ExpectedBalance        fixedpoint.Value
		BalanceCheckTorlerance fixedpoint.Value
		ActualBalance          fixedpoint.Value
		wantErr                bool
	}{
		{
			Currency:               "USDT",
			ExpectedBalance:        fixedpoint.NewFromFloat(1_000),
			BalanceCheckTorlerance: fixedpoint.NewFromFloat(0.05),
			ActualBalance:          fixedpoint.NewFromFloat(1_049),
			wantErr:                false,
		},
		{
			Currency:               "USDT",
			ExpectedBalance:        fixedpoint.NewFromFloat(1_000),
			BalanceCheckTorlerance: fixedpoint.NewFromFloat(0.05),
			ActualBalance:          fixedpoint.NewFromFloat(1_050),
			wantErr:                true,
		},
	}

	for _, tc := range cases {
		expectedBalances := map[string]fixedpoint.Value{
			tc.Currency: tc.ExpectedBalance,
		}
		bc := &BalanceCheck{
			ExpectedBalances:       expectedBalances,
			BalanceCheckTorlerance: tc.BalanceCheckTorlerance,
		}
		balances := types.BalanceMap{
			tc.Currency: types.Balance{
				Currency:  tc.Currency,
				Available: tc.ActualBalance,
			},
		}
		if tc.wantErr {
			assert.Error(t, bc.Check(balances))
		} else {
			assert.NoError(t, bc.Check(balances))
		}
	}
}

package testhelper

import (
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func BalancesFromText(str string) types.BalanceMap {
	balances := make(types.BalanceMap)
	lines := strings.Split(str, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		cols := strings.SplitN(line, ",", 2)
		if len(cols) < 2 {
			panic("column length should be 2")
		}

		currency := strings.TrimSpace(cols[0])
		available := fixedpoint.MustNewFromString(strings.TrimSpace(cols[1]))
		balances[currency] = Balance(currency, available)
	}

	return balances
}

// Balance returns a balance object with the given currency and available amount
func Balance(currency string, available fixedpoint.Value) types.Balance {
	return types.Balance{
		Currency:          currency,
		Available:         available,
		Locked:            fixedpoint.Zero,
		Borrowed:          fixedpoint.Zero,
		Interest:          fixedpoint.Zero,
		NetAsset:          fixedpoint.Zero,
		MaxWithdrawAmount: fixedpoint.Zero,
	}
}

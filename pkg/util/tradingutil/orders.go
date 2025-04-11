package tradingutil

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func HasSufficientBalance(
	baseCurrency string,
	quoteCurrency string,
	side types.SideType,
	price, quantity fixedpoint.Value,
	tradingMarket types.Market,
	balances types.BalanceMap,
) (bool, string) {
	// 1. check if the currency is correct
	_, okBase := balances[baseCurrency]
	_, okQuote := balances[quoteCurrency]
	if !okBase || !okQuote {
		return false, fmt.Sprintf("invalid currency paire: %s/%s", baseCurrency, quoteCurrency)
	}
	// 2. check if there is available balance
	switch side {
	case types.SideTypeSell:
		balance := balances[baseCurrency]
		if balance.Available.Compare(quantity) < 0 {
			return false, fmt.Sprintf(
				"insufficient balance for %s: %s < %s",
				baseCurrency,
				balance.Available.String(),
				quantity.String(),
			)
		}
	case types.SideTypeBuy:
		balance := balances[quoteCurrency]
		requiredQuote := price.Mul(quantity)
		if balance.Available.Compare(requiredQuote) < 0 {
			return false, fmt.Sprintf(
				"insufficient balance for %s: %s < %s(%s*%s)",
				quoteCurrency,
				balance.Available.String(),
				requiredQuote.String(),
				price.String(),
				quantity.String(),
			)
		}
	default:
		return false, fmt.Sprintf("invalid side %s", side)
	}
	return true, ""
}

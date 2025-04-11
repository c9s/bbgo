package tradingutil

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/types"
)

func HasSufficientBalance(
	tradingMarket types.Market,
	submitOrder types.SubmitOrder,
	balances types.BalanceMap,
) (bool, error) {
	// check if there is available balance by side
	switch submitOrder.Side {
	case types.SideTypeSell:
		return hasSufficientBaseForSell(
			tradingMarket.BaseCurrency,
			submitOrder,
			balances,
		)
	case types.SideTypeBuy:
		return hasSufficientQuoteForBuy(
			tradingMarket.QuoteCurrency,
			submitOrder,
			balances,
		)
	default:
		return false, fmt.Errorf("invalid side %s", submitOrder.Side)
	}
}

func hasSufficientBaseForSell(
	baseCurrency string,
	submitOrder types.SubmitOrder,
	balances types.BalanceMap,
) (bool, error) {
	balance, ok := balances[baseCurrency]
	if !ok {
		return false, fmt.Errorf("invalid base currency %s", baseCurrency)
	}
	if balance.Available.Compare(submitOrder.Quantity) < 0 {
		return false, fmt.Errorf(
			"insufficient balance for %s: %s < %s",
			baseCurrency,
			balance.Available.String(),
			submitOrder.Quantity.String(),
		)
	}
	return true, nil
}

func hasSufficientQuoteForBuy(
	quoteCurrency string,
	submitOrder types.SubmitOrder,
	balances types.BalanceMap,
) (bool, error) {
	balance, ok := balances[quoteCurrency]
	if !ok {
		return false, fmt.Errorf("invalid quote currency %s", quoteCurrency)
	}
	requiredQuote := submitOrder.Price.Mul(submitOrder.Quantity)
	if balance.Available.Compare(requiredQuote) < 0 {
		return false, fmt.Errorf(
			"insufficient balance for %s: %s < %s(%s*%s)",
			quoteCurrency,
			balance.Available.String(),
			requiredQuote.String(),
			submitOrder.Price.String(),
			submitOrder.Quantity.String(),
		)
	}
	return true, nil
}

package cmd

import (
	"github.com/c9s/bbgo/pkg/types"
)

func inBaseAsset(balances types.BalanceMap, market types.Market, price float64) float64 {
	quote := balances[market.QuoteCurrency]
	base := balances[market.BaseCurrency]
	return (base.Locked.Float64() + base.Available.Float64()) + ((quote.Locked.Float64() + quote.Available.Float64()) / price)
}


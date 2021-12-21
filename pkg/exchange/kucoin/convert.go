package kucoin

import (
	"github.com/c9s/bbgo/pkg/exchange/kucoin/kucoinapi"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalBalanceMap(accounts []kucoinapi.Account) types.BalanceMap {
	balances := types.BalanceMap{}

	// for now, we only return the trading account
	for _, account := range accounts {
		switch account.Type {
		case kucoinapi.AccountTypeTrade:
			balances[account.Currency] = types.Balance{
				Currency:  account.Currency,
				Available: account.Available,
				Locked:    account.Holds,
			}
		}
	}

	return balances
}

